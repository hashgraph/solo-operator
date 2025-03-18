// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitStatus;
import com.hedera.solo.operator.model.v1alpha1.OrbitStatusBuilder;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusCapsuleRole;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusConfigMap;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusHeadlessService;
import com.hedera.solo.operator.reconciler.orbit.resources.OrbitNamespace;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Workflow;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import jakarta.inject.Inject;
import java.time.Duration;
import java.time.Instant;
import java.time.format.DateTimeFormatter;
import org.jboss.logging.Logger;

@Workflow(
        dependents = {
            @Dependent(name = "namespace", type = OrbitNamespace.class),
            @Dependent(type = ConsensusConfigMap.class, dependsOn = "namespace"),
            @Dependent(type = ConsensusCapsuleRole.class, dependsOn = "namespace"),
            @Dependent(type = ConsensusHeadlessService.class, dependsOn = "namespace"),
        })
@ControllerConfiguration(name = "orbit")
public class OrbitReconciler implements Reconciler<Orbit> {

    private final Logger log;

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
            return UpdateControl.patchStatus(edit(new Orbit())
                    .with(metadata -> metadata.editMetadata()
                            .withName(orbit.getMetadata().getName())
                            .withNamespace(orbit.getMetadata().getNamespace()))
                    .with(status -> status.withStatus(createOrUpdateStatus(orbit)))
                    .build());
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

    private OrbitStatus createOrUpdateStatus(final Orbit orbit) {
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

        return builder.build();
    }
}
