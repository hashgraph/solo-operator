// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.KubernetesResourceList;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.javaoperatorsdk.operator.api.reconciler.Cleaner;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.LinkedList;
import java.util.List;
import java.util.Set;
import org.jboss.logging.Logger;

// NOTE: This class shouldn't implement Reconciler<NetworkOperation> interface as it shouldn't be picked up by the
// operator as a reconciler during test
/**
 * This class is a mock implementation of the Cleaner interface for processing NetworkOperation resources in sequence.
 */
public class SequenceReconcilerMock implements Cleaner<NetworkOperation> {
    private static final Logger log = Logger.getLogger(SequenceReconcilerMock.class);
    private static final Set<NetworkOperationStatus.Phase> DONE =
            Set.of(NetworkOperationStatus.Phase.Failed, NetworkOperationStatus.Phase.Succeeded);

    private static KubernetesClient client;
    private static final Sequencer<NetworkOperation> sequencer =
            new Sequencer<>(SequenceReconcilerMock::activeResources);

    /**
     * Constructor for SequenceReconcilerMock. Initializes the Kubernetes client and sets up the sequence reconciler.
     *
     * @param client The KubernetesClient instance to be used for interacting with the Kubernetes cluster.
     */
    @Inject
    public SequenceReconcilerMock(KubernetesClient client) {
        SequenceReconcilerMock.client = client;
    }

    /**
     * Processes a NetworkOperation resource and sets its desired status and phase.
     *
     * @param resource The NetworkOperation resource to be processed.
     * @param context The Context object containing information about the Kubernetes cluster and other relevant data.
     * @return A new instance of NetworkOperation with updated status and phase set to succeeded
     * @throws Exception If an error occurs during processing.
     */
    public NetworkOperation process(final NetworkOperation resource, final Context<NetworkOperation> context)
            throws Exception {

        // process and set desired status and phase
        log.infof(
                "Processing'%s/%s' [created: %s]",
                resource.getMetadata().getNamespace(),
                resource.getMetadata().getName(),
                resource.getMetadata().getCreationTimestamp());

        Thread.sleep(100);

        // set status succeeded and return resource
        return edit(new NetworkOperation())
                .with(metadata -> metadata.editMetadata()
                        .withName(resource.getMetadata().getName())
                        .withNamespace(resource.getMetadata().getNamespace())
                        .withUid(resource.getMetadata().getUid()))
                .with(r -> r.editStatus()
                        .withPhase(NetworkOperationStatus.Phase.Succeeded)
                        .endStatus())
                .build();
    }

    /**
     * Reconciles the NetworkOperation resource by processing it and setting its status based on the sequence response.
     *
     * @param resource The NetworkOperation resource to be reconciled.
     * @param context The Context object containing information about the Kubernetes cluster and other relevant data.
     * @return An UpdateControl object that can be used to patch or reschedule the reconciliation process
     * @throws Exception If an error occurs during reconciliation.
     */
    public UpdateControl<NetworkOperation> reconcile(
            final NetworkOperation resource, final Context<NetworkOperation> context) throws Exception {
        // is this resource done, then return
        if (isDone(resource)) {
            log.infof(
                    "Done '%s/%s' [created: %s]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getMetadata().getCreationTimestamp());
            return UpdateControl.noUpdate();
        }

        SequencerResponse<NetworkOperation> response =
                sequencer.checkNext(new SequencerRequest<>(resource, Duration.ofSeconds(2), Duration.ofSeconds(1)));
        if (!response.firstInQueue()) {
            log.infof(
                    "Not first in queue '%s/%s' [created: %s]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getMetadata().getCreationTimestamp());

            if (response.shouldReschedule()) {
                return UpdateControl.<NetworkOperation>noUpdate().rescheduleAfter(response.reconcileDelay());
            }

            return UpdateControl.noUpdate();
        }

        NetworkOperation processed = process(resource, context);
        if (isDone(processed)) {
            sequencer.markAsCompleted(new SequencerRequest<>(processed));
        }

        return UpdateControl.patchStatus(processed).rescheduleAfter(response.reconcileDelay());
    }

    /**
     * Prepares and returns a list of active NetworkOperation resources that need to be considered for sequencing.
     *
     * @return A List of active NetworkOperation resources
     */
    public static List<NetworkOperation> activeResources() {
        MixedOperation<NetworkOperation, KubernetesResourceList<NetworkOperation>, Resource<NetworkOperation>>
                resources = client.resources(NetworkOperation.class);

        // select active resources
        final List<NetworkOperation> activeItems = new LinkedList<>();
        for (NetworkOperation r : resources.list().getItems()) {
            if (!isDone(r)) {
                activeItems.add(r);
            }
        }

        return activeItems;
    }

    /**
     * Checks if a NetworkOperation resource is done based on its status.
     *
     * @param resource The NetworkOperation resource to check.
     * @return True if the resource is done, false otherwise
     */
    private static boolean isDone(NetworkOperation resource) {
        return resource.getStatus() != null
                && DONE.contains(resource.getStatus().getPhase());
    }

    @Override
    public DeleteControl cleanup(NetworkOperation resource, Context<NetworkOperation> context) {
        sequencer.markAsDeleted(new SequencerRequest<>(resource));
        return DeleteControl.defaultDelete();
    }
}
