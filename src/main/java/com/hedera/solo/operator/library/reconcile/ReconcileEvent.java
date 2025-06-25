// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.reconcile;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.generic.Nullable.orEmptyList;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import io.fabric8.kubernetes.api.model.Event;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.ObjectReference;
import io.fabric8.kubernetes.api.model.StatusCause;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import io.javaoperatorsdk.operator.AggregatedOperatorException;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

/** Utility class for creating or updating Kubernetes events to track the status of resource reconciliation. */
public class ReconcileEvent {
    /** Private constructor to prevent instantiation of this utility class. */
    private ReconcileEvent() {}

    /**
     * Creates an event for a resource if it does not exist, or updates an existing event if it does. The event is
     * marked as an error with the reason "Failed" and includes a description of the exception.
     *
     * @param <P> The type of the Kubernetes resource being reconciled.
     * @param resource The Kubernetes resource being reconciled.
     * @param context The operator context used to interact with the Kubernetes API.
     * @param exception The exception that occurred during reconciliation, if any.
     */
    public static <P extends HasMetadata> void createOrUpdateEvent(
            final P resource, final Context<P> context, final Exception exception) {
        if (exception instanceof AggregatedOperatorException ae) {
            ae.getAggregatedExceptions().values().forEach(ex -> createOrUpdateEvent(resource, context, ex));
            return;
        }
        if (exception instanceof KubernetesClientException ke) {
            if (orEmptyList(() -> ke.getStatus().getDetails().getCauses()).stream()
                    .map(StatusCause::getReason)
                    .anyMatch("NamespaceTerminating"::equals)) {
                return;
            }
        }
        KubernetesClient client = context.getClient();
        String timestamp = timestamp();
        ObjectMeta metadata = resource.getMetadata();
        List<Event> events = client.resources(Event.class)
                .inNamespace(resource.getMetadata().getNamespace())
                .withInvolvedObject(new ObjectReference(
                        resource.getApiVersion(),
                        null,
                        resource.getKind(),
                        metadata.getName(),
                        metadata.getNamespace(),
                        null,
                        metadata.getUid()))
                .list()
                .getItems();
        if (events.isEmpty()) {
            Event event = edit(new Event())
                    .with(builder -> builder.editMetadata()
                            .withGenerateName(resource.getMetadata().getName() + "-")
                            .withNamespace(resource.getMetadata().getNamespace()))
                    .with(object -> object.editInvolvedObject()
                            .withApiVersion(resource.getApiVersion())
                            .withKind(resource.getKind())
                            .withUid(resource.getMetadata().getUid())
                            .withName(resource.getMetadata().getName())
                            .withNamespace(resource.getMetadata().getNamespace()))
                    .with(builder -> builder.withCount(1)
                            .withType("Error")
                            .withReason("Failed")
                            .withMessage(describe(exception))
                            .withReportingComponent("solo-operator")
                            .withFirstTimestamp(timestamp)
                            .withLastTimestamp(timestamp))
                    .build();
            client.resource(event).create();
        } else {
            Event event = events.getFirst();
            client.resource(edit(event)
                            .with(builder ->
                                    builder.withCount(event.getCount() + 1).withLastTimestamp(timestamp))
                            .build())
                    .patch();
        }
    }

    /**
     * Cleans up events associated with a Kubernetes resource.
     *
     * @param <P> The type of the Kubernetes resource.
     * @param resource The Kubernetes resource to clean up events for.
     * @param context The operator context used to interact with the Kubernetes API.
     */
    public static <P extends HasMetadata> void cleanupEvents(final P resource, final Context<P> context) {
        ObjectMeta metadata = resource.getMetadata();
        (metadata.getNamespace() == null
                        ? context.getClient().resources(Event.class)
                        : context.getClient().resources(Event.class).inNamespace(metadata.getNamespace()))
                .withInvolvedObject(new ObjectReference(
                        resource.getApiVersion(),
                        null,
                        resource.getKind(),
                        metadata.getName(),
                        metadata.getNamespace(),
                        null,
                        metadata.getUid()))
                .delete();
    }

    /**
     * Describes an exception in a human-readable format for logging or event messages.
     *
     * @param e The exception to describe.
     * @return A string representation of the exception including its type and message
     */
    public static String describe(final Exception e) {
        StringBuilder sb = new StringBuilder(e.getClass().getSimpleName());
        if (sb.isEmpty()) {
            sb.append(e.getClass().getName());
        }
        sb.append(Arrays.stream(e.getStackTrace())
                .limit(3)
                .map(ste -> ste.getMethodName() + "(" + ste.getFileName() + ":" + ste.getLineNumber() + ")")
                .collect(Collectors.joining(", ", "[", "]")));
        sb.append(": ");
        sb.append(e.getMessage());
        return sb.toString();
    }
}
