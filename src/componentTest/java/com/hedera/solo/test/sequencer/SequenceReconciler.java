// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.sequencer;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.sequencer.Sequencer;
import com.hedera.solo.operator.library.sequencer.SequencerRequest;
import com.hedera.solo.operator.library.sequencer.SequencerResponse;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.KubernetesResourceList;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.LinkedList;
import java.util.List;
import java.util.Set;
import org.jboss.logging.Logger;

@ControllerConfiguration
public class SequenceReconciler implements Reconciler<NetworkOperation> {
    private static final Set<NetworkOperationStatus.Phase> DONE =
            Set.of(NetworkOperationStatus.Phase.Failed, NetworkOperationStatus.Phase.Succeeded);

    private final Logger log;
    private static KubernetesClient client;
    private static final Sequencer<NetworkOperation> sequencer = new Sequencer<>(SequenceReconciler::activeResources);

    @Inject
    public SequenceReconciler(final Logger log, KubernetesClient client) {
        this.log = log;
        SequenceReconciler.client = client;
    }

    @Override
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
                    "Not first in queue '%s/%s' [created: %s, rescheduleAfter: %s]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getMetadata().getCreationTimestamp(),
                    response.reconcileDelay().toMillis());

            NetworkOperation desired = edit(resource)
                    .with(r -> r.editStatus()
                            .withPhase(NetworkOperationStatus.Phase.Pending)
                            .endStatus())
                    .build();

            return UpdateControl.patchStatus(desired).rescheduleAfter(response.reconcileDelay());
        }

        // process and set desired status and phase
        Thread.sleep(100);
        log.infof(
                "Processed '%s/%s' [created: %s]",
                resource.getMetadata().getNamespace(),
                resource.getMetadata().getName(),
                resource.getMetadata().getCreationTimestamp());
        NetworkOperation desired = edit(resource)
                .with(r -> r.editStatus()
                        .withPhase(NetworkOperationStatus.Phase.Succeeded)
                        .endStatus())
                .build();

        // if done, mark as processed
        if (isDone(desired)) {
            sequencer.markAsProcessed(new SequencerRequest<>(desired));
            return UpdateControl.patchStatus(desired);
        }

        return UpdateControl.patchStatus(desired)
                .rescheduleAfter(sequencer.computeReconcileDelay(new SequencerRequest<>(desired)));
    }

    /** Prepare and return the list of active resources that needs to be considered for sequencing. */
    public static List<NetworkOperation> activeResources() {
        MixedOperation<NetworkOperation, KubernetesResourceList<NetworkOperation>, Resource<NetworkOperation>>
                resources = client.resources(NetworkOperation.class);

        // select active resources
        final List<NetworkOperation> activeItems = new LinkedList<>();
        for (NetworkOperation r : resources.inAnyNamespace().list().getItems()) {
            if (r.getStatus() == null || !isDone(r)) {
                activeItems.add(r);
            }
        }

        return activeItems;
    }

    private static boolean isDone(NetworkOperation resource) {
        return resource.getStatus() != null
                && DONE.contains(resource.getStatus().getPhase());
    }
}
