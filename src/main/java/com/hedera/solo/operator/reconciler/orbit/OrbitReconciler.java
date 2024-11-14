// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitStatus;
import com.hedera.solo.operator.model.v1alpha1.OrbitStatusBuilder;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusCapsuleRole;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusConfigMap;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusHeadlessService;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusPublicKeys;
import com.hedera.solo.operator.reconciler.orbit.resources.OrbitNamespace;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import jakarta.inject.Inject;
import java.time.Duration;
import java.time.Instant;
import java.time.format.DateTimeFormatter;
import java.util.Map;
import org.jboss.logging.Logger;

@ControllerConfiguration(
        name = "orbit"
//        dependents = {
//            @Dependent(name = "namespace", type = OrbitNamespace.class),
//            @Dependent(type = ConsensusConfigMap.class, dependsOn = "namespace"),
//            @Dependent(type = ConsensusCapsuleRole.class, dependsOn = "namespace"),
//            @Dependent(type = ConsensusHeadlessService.class, dependsOn = "namespace"),
//            @Dependent(type = ConsensusPublicKeys.class, dependsOn = "namespace"),
//        }
        )
public class OrbitReconciler implements EventSourceInitializer<Orbit>, Reconciler<Orbit>, ErrorStatusHandler<Orbit> {

    private final Logger log;
    private final OrbitNamespace orbitNamespace = new OrbitNamespace();
    private final ConsensusCapsuleRole consensusCapsuleRole = new ConsensusCapsuleRole();
    private final ConsensusConfigMap consensusConfigMap = new ConsensusConfigMap();
    private final ConsensusHeadlessService consensusHeadlessService = new ConsensusHeadlessService();
    private final ConsensusPublicKeys consensusPublicKeys = new ConsensusPublicKeys();
    private final Workflow<Orbit> workflow = new WorkflowBuilder<Orbit>()
            .addDependentResource(orbitNamespace)
            .addDependentResource(consensusCapsuleRole).dependsOn(orbitNamespace)
            .addDependentResource(consensusConfigMap).dependsOn(orbitNamespace)
            .addDependentResource(consensusHeadlessService).dependsOn(orbitNamespace)
            .addDependentResource(consensusPublicKeys).dependsOn(orbitNamespace)
            .build();

    @Inject
    public OrbitReconciler(final Logger log) {
        this.log = log;
    }

    @Override
    public UpdateControl<Orbit> reconcile(final Orbit orbit, final Context<Orbit> context) throws Exception {
        try {
            log.infof(
                    "Orbit - Starting Reconciliation [ name = '%s' ]",
                    orbit.getMetadata().getName());
            workflow.reconcile(orbit, context);
            createOrUpdateStatus(orbit);
            return UpdateControl.patchStatus(orbit);
        } finally {
            log.infof(
                    "Orbit - Finished Reconciliation [ name = '%s' ]",
                    orbit.getMetadata().getName());
        }
    }

    @Override
    public ErrorStatusUpdateControl<Orbit> updateErrorStatus(
            final Orbit orbit, final Context<Orbit> context, final Exception e) {
        return ErrorStatusUpdateControl.patchStatus(orbit);
    }

    private void createOrUpdateStatus(final Orbit orbit) {
        final OrbitStatusBuilder builder =
                orbit.getStatus() != null ? orbit.getStatus().edit() : new OrbitStatusBuilder();
        final Instant now = Instant.now();

        builder.withPhase(OrbitStatus.Phase.Running);

        if (!builder.hasStartTime()) {
            builder.withStartTime(DateTimeFormatter.ISO_INSTANT.format(now));
        }

        if (!builder.hasExpirationTime()
                && orbit.getSpec().getLifetime() != null
                && !orbit.getSpec().getLifetime().isEmpty()) {
            final Duration dur = Duration.parse(orbit.getSpec().getLifetime());
            builder.withExpirationTime(DateTimeFormatter.ISO_INSTANT.format(dur.addTo(now)));
        }

        builder.addToConditions(new ConditionBuilder()
                .withType("Ready")
                .withStatus(ConditionStatus.True)
                .withLastTransitionTime(DateTimeFormatter.ISO_INSTANT.format(now))
                .build());

        builder.withObservedGeneration(orbit.getMetadata().getGeneration());

        orbit.setStatus(builder.build());
    }

    @Override
    public Map<String, EventSource> prepareEventSources(EventSourceContext<Orbit> context) {
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
