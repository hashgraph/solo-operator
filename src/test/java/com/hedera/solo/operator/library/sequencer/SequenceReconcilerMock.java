// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

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
import org.jboss.logging.Logger;

import java.time.Duration;
import java.util.LinkedList;
import java.util.List;
import java.util.Set;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

// NOTE: This class shouldn't implement Reconciler<NetworkOperation> interface as it shouldn't be picked up by the
// operator as a reconciler during test
public class SequenceReconcilerMock implements Cleaner<NetworkOperation> {
    private static final Logger log = Logger.getLogger(SequenceReconcilerMock.class);
    private static final Set<NetworkOperationStatus.Phase> DONE =
            Set.of(NetworkOperationStatus.Phase.Failed, NetworkOperationStatus.Phase.Succeeded);

    private static KubernetesClient client;
    private static final Sequencer<NetworkOperation> sequencer =
            new Sequencer<>(SequenceReconcilerMock::activeResources);

    @Inject
    public SequenceReconcilerMock(KubernetesClient client) {
        SequenceReconcilerMock.client = client;
    }

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
        return edit(resource)
                .with(r -> r.editStatus()
                        .withPhase(NetworkOperationStatus.Phase.Succeeded)
                        .endStatus())
                .build();
    }

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
     * Prepare and return the list of active resources that needs to be considered for sequencing.
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
