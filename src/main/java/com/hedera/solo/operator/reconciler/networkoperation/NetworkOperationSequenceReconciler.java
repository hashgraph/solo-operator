// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec.Mode.NodeAdd;
import static com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec.Mode.NodeDelete;
import static com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec.Mode.NodeUpdate;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.library.sequencer.Sequenced;
import com.hedera.solo.operator.library.sequencer.Sequencer;
import com.hedera.solo.operator.library.sequencer.SequencerRequest;
import com.hedera.solo.operator.library.sequencer.SequencerResponse;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.NetworkGenesisProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.node.NodeOperationProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.NetworkUpgradeProcessor;
import io.javaoperatorsdk.operator.api.reconciler.Cleaner;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.ArrayList;
import java.util.LinkedList;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import org.jboss.logging.Logger;

/** {@code NetworkOperationReconciler} operates on a single CR per namespace at any given time. */
@ControllerConfiguration(name = "network-operation-v2")
public class NetworkOperationSequenceReconciler implements Reconciler<NetworkOperation>, Cleaner<NetworkOperation> {
    public static final String VERSION = "v2";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private static final Sequencer<NetworkOperation> sequencer =
            new Sequencer<>(NetworkOperationSequenceReconciler::activeResources);

    private static final Set<NetworkOperationStatus.Phase> DONE =
            Set.of(NetworkOperationStatus.Phase.Failed, NetworkOperationStatus.Phase.Succeeded);

    public static final Duration DEFAULT_RESCHEDULE_DELAY = Duration.ofSeconds(1);

    private static CustomResourceManager customResourceManager;

    private final NetworkGenesisProcessor networkGenesisProcessor;

    private final NetworkUpgradeProcessor networkUpgradeProcessor;

    private final NetworkCheckProcessor networkCheckProcessor;

    private final NodeOperationProcessor nodeOperationProcessor;

    @Inject
    public NetworkOperationSequenceReconciler(
            CustomResourceManager customResourceManager,
            NetworkGenesisProcessor networkGenesisProcessor,
            NetworkUpgradeProcessor networkUpgradeProcessor,
            NetworkCheckProcessor networkCheckProcessor,
            NodeOperationProcessor nodeOperationProcessor) {
        NetworkOperationSequenceReconciler.customResourceManager = customResourceManager;
        this.networkGenesisProcessor = networkGenesisProcessor;
        this.networkUpgradeProcessor = networkUpgradeProcessor;
        this.networkCheckProcessor = networkCheckProcessor;
        this.nodeOperationProcessor = nodeOperationProcessor;
    }

    /**
     * Process the network operation based on the mode and spec
     *
     * @param resource the network operation resource
     * @param context the context
     * @return the processed network operation with desired status
     * @throws Exception if an error occurs during processing
     */
    public ProcessorResult<NetworkOperationStatus> process(
            final NetworkOperation resource, final Context<NetworkOperation> context) throws Exception {
        log.infof(
                "'%s/%s' [mode: %s, created: %s, uuid: %s, phase: %s]: PROCESSING",
                resource.getMetadata().getNamespace(),
                resource.getMetadata().getName(),
                resource.getSpec().getMode(),
                resource.getMetadata().getCreationTimestamp(),
                resource.getMetadata().getUid(),
                resource.getStatus().getPhase());

        ProcessorResult<NetworkOperationStatus> result =
                processorFromMode(resource).process(resource, context);
        if (Objects.nonNull(result.getDeferred())) {
            @SuppressWarnings("unchecked")
            final var deferred =
                    (Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>>)
                            result.getDeferred();
            result = deferred.process(resource, context);
        }

        return result;
    }

    /**
     * Reconcile the network operation resource in sequence of creation time order
     *
     * <p>In practice, network operation CR should be created with a unique name in chronological order. If multiple CRs
     * are created at the same time, then the order of reconciliation is determined based on the name of the CR. This
     * sequencing is handled in the {@link Sequencer} class.
     *
     * @param resource the network operation resource
     * @param context the context
     * @return the update control with the network operation resource
     * @throws Exception if an error occurs during reconciliation
     */
    @Override
    public UpdateControl<NetworkOperation> reconcile(
            final NetworkOperation resource, final Context<NetworkOperation> context) throws Exception {
        // if the version is not v2, then skip
        if (!resource.getSpec().getVersion().equals(VERSION)) {
            log.infof(
                    " '%s/%s' [created: %s, version: %s, uuid: %s]: SKIPPED",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getMetadata().getCreationTimestamp(),
                    resource.getSpec().getVersion(),
                    resource.getMetadata().getName());
            return UpdateControl.noUpdate();
        }

        // if it hasn't been initialized yet, then initialize and reschedule so that initialized resource is persisted.
        // note, it initializes a new status object with consensus capsule names and phase as "Pending"
        if (resource.getStatus() == null
                || resource.getStatus().getConsensusCapsules() == null
                || resource.getStatus().getConsensusCapsules().isEmpty()) {
            final NetworkOperation initialized = initialize(resource);
            return UpdateControl.patchStatus(initialized).rescheduleAfter(DEFAULT_RESCHEDULE_DELAY);
        }

        // is this resource is done, then return
        if (isDone(resource)) {
            log.infof(
                    "'%s/%s' [created: %s, uuid: %s]: DONE",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getMetadata().getCreationTimestamp(),
                    resource.getMetadata().getUid());
            return UpdateControl.noUpdate();
        }

        // check if this resource is first in the queue
        SequencerResponse<NetworkOperation> response =
                sequencer.checkNext(new SequencerRequest<>(resource, Duration.ofSeconds(30), Duration.ofMinutes(1)));
        if (!response.firstInQueue()) {
            final List<String> itemSequence = new ArrayList<>();
            for (Sequenced item : sequencer.itemSequence(resource.getMetadata().getNamespace())) {
                itemSequence.add(item.name());
            }

            log.infof(
                    "'%s/%s' [created: %s, uuid: %s, queue: %s]: NOT FIRST IN QUEUE",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getMetadata().getCreationTimestamp(),
                    resource.getMetadata().getUid(),
                    itemSequence.toString());
            if (response.shouldReschedule()) {
                return UpdateControl.<NetworkOperation>noUpdate().rescheduleAfter(response.reconcileDelay());
            }

            return UpdateControl.noUpdate();
        }

        // process the resource and set the desired status
        final ProcessorResult<NetworkOperationStatus> result = process(resource, context);
        NetworkOperation desired =
                edit(resource).with(r -> r.withStatus(result.getStatus())).build();
        if (isDone(desired)) {
            log.infof(
                    "'%s/%s' [mode: %s, created: %s, uuid: %s]: MARK AS DONE",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getMode(),
                    resource.getMetadata().getCreationTimestamp(),
                    resource.getMetadata().getUid());
            sequencer.markAsCompleted(new SequencerRequest<>(desired));
            return UpdateControl.patchStatus(desired).rescheduleAfter(DEFAULT_RESCHEDULE_DELAY);
        }

        final Duration delay = sequencer.computeReconcileDelay(
                new SequencerRequest<>(desired, result.getReconcileDelay(), result.getProcessingTime()));
        return UpdateControl.patchStatus(desired).rescheduleAfter(delay);
    }

    /**
     * Initialize the resource with a new status object with consensus capsule names and phase as "Pending"
     *
     * @param resource the resource to be initialized
     * @return the initialized resource
     */
    private NetworkOperation initialize(final NetworkOperation resource) {
        final String namespace = resource.getMetadata().getNamespace();
        final List<ConsensusCapsule> capsules = customResourceManager.consensusCapsules(namespace);
        final List<String> capsuleNames = new LinkedList<>();
        for (ConsensusCapsule capsule : capsules) {
            capsuleNames.add(capsule.getMetadata().getName());
        }

        return edit(resource)
                .with(r -> r.withNewStatus() // initializing an empty new status
                        .withConsensusCapsules(capsuleNames)
                        .withPhase(NetworkOperationStatus.Phase.Pending))
                .build();
    }

    /**
     * Return the processor based on the mode of the network operation
     *
     * @param resource the network operation resource
     */
    private Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>>
            processorFromMode(final NetworkOperation resource) {
        Objects.requireNonNull(resource, "Network operation resource cannot be null");
        Objects.requireNonNull(resource.getSpec(), "Network operation spec cannot be null");
        Objects.requireNonNull(resource.getSpec().getMode(), "Network operation mode cannot be null");

        // return the processor based on the mode of the network operation
        if (resource.getSpec().getMode().equals(NetworkOperationSpec.Mode.NetworkGenesis)) {
            return networkGenesisProcessor;
        } else if (resource.getSpec().getMode().equals(NetworkOperationSpec.Mode.NetworkUpgrade)) {
            return networkUpgradeProcessor;
        } else if (resource.getSpec().getMode().equals(NetworkOperationSpec.Mode.NetworkCheck)) {
            return networkCheckProcessor;
        } else if (resource.getSpec().getMode().equals(NodeAdd)
                || resource.getSpec().getMode().equals(NodeDelete)
                || resource.getSpec().getMode().equals(NodeUpdate)) {
            return nodeOperationProcessor;
        } else {
            throw new IllegalArgumentException(
                    "Unknown network operation mode: " + resource.getSpec().getMode());
        }
    }

    /** Prepare and return the list of active resources that needs to be considered for sequencing. */
    public static List<NetworkOperation> activeResources() {
        List<NetworkOperation> items = customResourceManager.networkOperations();
        List<NetworkOperation> active = new ArrayList<>();
        for (NetworkOperation item : items) {
            if (!isDone(item)) {
                active.add(item);
            }
        }
        return active;
    }

    /**
     * Check if the resource is done
     *
     * <p>If the phase is Failed or Succeeded, it is considered done.
     *
     * @param resource the resource to be checked
     * @return true if the resource is done, false otherwise
     */
    private static boolean isDone(NetworkOperation resource) {
        return resource.getStatus() != null
                && resource.getStatus().getConsensusCapsules() != null
                && !resource.getStatus().getConsensusCapsules().isEmpty() // check it is initialized
                && DONE.contains(resource.getStatus().getPhase());
    }

    /**
     * Cleanup the resource by marking it for deletion
     *
     * @param resource the resource that is marked for deletion
     * @param context the context with which the operation is executed
     */
    @Override
    public DeleteControl cleanup(NetworkOperation resource, Context<NetworkOperation> context) {
        sequencer.markAsDeleted(new SequencerRequest<>(resource));
        return DeleteControl.defaultDelete();
    }
}
