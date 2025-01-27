// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.library.sequencer.AbstractSequenceReconciler;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.NetworkGenesisProcessor;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowReconcileResult;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collection;
import java.util.Comparator;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import org.jboss.logging.Logger;

/** {@code NetworkOperationReconciler} operates on a single CR per namespace at any given time. */
@ControllerConfiguration(name = "network-operation-v2")
public class NetworkOperationReconcilerV2
        extends AbstractSequenceReconciler<NetworkOperation, NetworkOperationSpec, NetworkOperationStatus>
        implements EventSourceInitializer<NetworkOperation>, Reconciler<NetworkOperation> {
    public static final String VERSION = "v2";
    public static final String NAME = "network-operation-v2";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    // prepare mediator and dependents
    private static final NetworkOperationMediator mediator = new NetworkOperationMediator();

    // prepare processors
    private final NetworkGenesisProcessor networkGenesisProcessor = new NetworkGenesisProcessor();
    private final NetworkCheckProcessor networkCheckProcessor = new NetworkCheckProcessor();

    @Override
    public String getName() {
        return NAME;
    }

    @Override
    public String getOrbitName(NetworkOperation resource) {
        return resource.getSpec().getOrbit();
    }

    @Override
    public Long getSequenceNumber(NetworkOperation resource) {
        Objects.requireNonNull(resource, "resource must not be null");
        if (Objects.nonNull(resource.getStatus())) {
            return resource.getStatus().getSequenceNumber();
        }

        return null;
    }

    @Override
    public NetworkOperation setSequenceNumber(
            NetworkOperation resource, Context<NetworkOperation> context, Long sequenceNumber) {
        return resource.edit()
                .editStatus()
                .withSequenceNumber(sequenceNumber)
                .withPhase(NetworkOperationStatus.Phase.Pending)
                .endStatus()
                .build();
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkOperation> context) {
        return mediator.prepareEventSources(context);
    }

    @Override
    protected Duration computeReconcileDelay(
            NetworkOperation resource, ProcessorResult<NetworkOperationStatus> result) {
        Duration delay = Duration.ofSeconds(1);
        if (Objects.nonNull(result)) {
            delay = result.getReconcileDelay();
            // FIXME add additional logic to determine the delay
        }
        return delay;
    }

    @Override
    protected boolean shouldProcess(NetworkOperation resource, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "resource must not be null");
        Objects.requireNonNull(resource.getSpec(), "spec must not be null");
        Objects.requireNonNull(resource.getSpec().getVersion(), "version must not be null");
        return resource.getSpec().getVersion().equals(VERSION); // only process v2
    }

    @Override
    public UpdateControl<NetworkOperation> process(
            final NetworkOperation resource, final Context<NetworkOperation> context) throws Exception {
        NetworkOperation desired = null;
        try {
            log.infof(
                    "Network Initialization - Starting Reconciliation [ name = '%s' version = '%s' ]",
                    resource.getMetadata().getName(), resource.getSpec().getVersion());

            // reconcile dependent resources through a custom workflow
            WorkflowReconcileResult reconcileResult = mediator.reconcileWorkflow(resource, context);
            if (reconcileResult.allDependentResourcesReady()) {
                log.infof(
                        "Network Initialization - Dependence resources are ready [ name = '%s' version = '%s' ]",
                        resource.getMetadata().getName(), resource.getSpec().getVersion());
            } else {
                log.warnf(
                        "Network Initialization - Dependence resources are not ready [ name = '%s' version = '%s' ]",
                        resource.getMetadata().getName(), resource.getSpec().getVersion());
            }

            final var processor = processorFromMode(resource, context);
            ProcessorResult<NetworkOperationStatus> result = processor.process(resource, context);
            if (Objects.nonNull(result.getDeferred())) {
                @SuppressWarnings("unchecked")
                final var deferred =
                        (Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>>)
                                result.getDeferred();
                result = deferred.process(resource, context);
            }

            // compute the desired resource and schedule with appropriate reconciliation delay
            desired = prepareDesired(resource, context, result);
            return UpdateControl.patchStatus(desired).rescheduleAfter(computeReconcileDelay(desired, result));

        } finally {
            if (Objects.isNull(desired) || desired.getStatus().getPhase().equals(NetworkOperationStatus.Phase.Failed)) {
                log.warnf(
                        "Network Initialization - Failed Reconciliation [ name = '%s', version = '%s' ]",
                        resource.getMetadata().getName(), resource.getSpec().getVersion());
            }

            log.infof(
                    "Network Initialization - Finished Reconciliation [ name = '%s', version = '%s' ]",
                    resource.getMetadata().getName(), resource.getSpec().getVersion());
        }
    }

    private Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>>
            processorFromMode(NetworkOperation resource, Context<NetworkOperation> context) {
        // FIXME add support for other modes and processors
        switch (resource.getSpec().getMode()) {
            case NetworkOperationSpec.Mode.NetworkGenesis:
                return networkGenesisProcessor;
            case NetworkOperationSpec.Mode.NetworkCheck:
                return networkCheckProcessor;
            default:
                throw new IllegalArgumentException(
                        "Unknown mode: " + resource.getSpec().getMode());
        }
    }

    private NetworkOperation prepareDesired(
            NetworkOperation resource,
            Context<NetworkOperation> context,
            ProcessorResult<NetworkOperationStatus> result) {

        // FIXME no need to determine phase here, move to NetworkCheck processor and just use the returned status for
        // the desired resource
        final NetworkOperationStatus status = result.getStatus();
        Collection<ConsensusCapsule> consensusCapsules =
                mediator.findConsensusCapsules(resource, context).values();
        return edit(resource)
                .with(r -> r.withStatus(status))
                .with(r -> r.editStatus()
                        .withObservedGeneration(resource.getMetadata().getGeneration()))
                .with(determinePhase(consensusCapsules))
                .build();
    }

    // collect consensus capsule status in status until any fail or all are running.
    // FIXME move this to NetworkCheck processor
    private BuilderFunction<NetworkOperationBuilder> determinePhase(Collection<ConsensusCapsule> consensusCapsules) {
        List<ConsensusCapsuleStatus.Phase> phaseList = consensusCapsules.stream()
                .map(CustomResource::getStatus)
                .filter(Objects::nonNull)
                .map(ConsensusCapsuleStatus::getPhase)
                .filter(Objects::nonNull)
                .toList();
        Set<ConsensusCapsuleStatus.Phase> phases = new HashSet<>(phaseList);
        NetworkOperationStatus.Phase phase;
        List<Condition> conditions = new ArrayList<>();

        if (Set.of(ConsensusCapsuleStatus.Phase.Running).equals(phases)
                && consensusCapsules.size() == phaseList.size()) {
            phase = NetworkOperationStatus.Phase.Succeeded;
            conditions.add(new ConditionBuilder()
                    .withType("Ready")
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(Instant.now().toString())
                    .build());
        } else if (Set.of(
                        ConsensusCapsuleStatus.Phase.Pending,
                        ConsensusCapsuleStatus.Phase.Running,
                        ConsensusCapsuleStatus.Phase.Unknown)
                .containsAll(phases)) {
            phase = NetworkOperationStatus.Phase.Running;
        } else {
            phase = NetworkOperationStatus.Phase.Failed;
            Map<String, ConsensusCapsuleStatus.Phase> map = consensusCapsules.stream()
                    .filter(c -> Objects.nonNull(c.getStatus())
                            && Objects.nonNull(c.getStatus().getPhase()))
                    .sorted(Comparator.comparing(c -> c.getMetadata().getName()))
                    .collect(
                            LinkedHashMap::new,
                            (m, c) -> m.put(
                                    c.getMetadata().getName(), c.getStatus().getPhase()),
                            Map::putAll);
            conditions.add(new ConditionBuilder()
                    .withType("Ready")
                    .withStatus(ConditionStatus.False)
                    .withReason("CapsulePhase")
                    .withMessage("Capsule phases: " + map)
                    .withLastTransitionTime(Instant.now().toString())
                    .build());
        }

        return builder -> builder.editStatus().withPhase(phase).addAllToConditions(conditions);
    }
}
