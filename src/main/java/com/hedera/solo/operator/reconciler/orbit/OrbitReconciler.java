/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.reconciler.orbit;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitStatus;
import com.hedera.solo.operator.model.v1alpha1.OrbitStatusBuilder;
import com.hedera.solo.operator.reconciler.orbit.dependent.OrbitConfigMap;
import com.hedera.solo.operator.reconciler.orbit.dependent.OrbitNamespace;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import jakarta.inject.Inject;
import java.time.Duration;
import java.time.Instant;
import java.time.format.DateTimeFormatter;
import org.jboss.logging.Logger;

@ControllerConfiguration(
        name = "orbit",
        dependents = {
            @Dependent(name = "namespace", type = OrbitNamespace.class),
            @Dependent(type = OrbitConfigMap.class, dependsOn = "namespace"),
        })
public class OrbitReconciler implements Reconciler<Orbit>, ErrorStatusHandler<Orbit> {

    private final KubernetesClient client;

    private final Logger log;

    @Inject
    public OrbitReconciler(final KubernetesClient client, final Logger log) {
        this.client = client;
        this.log = log;
    }

    @Override
    public UpdateControl<Orbit> reconcile(final Orbit orbit, final Context<Orbit> context) throws Exception {
        try {
            log.infof(
                    "Orbit - Starting Reconciliation [ name = '%s' ]",
                    orbit.getMetadata().getName());
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
        log.errorf(
                e,
                "Orbit - Error During Reconciliation [ name = '%s' ]",
                orbit.getMetadata().getName());
        return ErrorStatusUpdateControl.patchStatus(orbit);
    }

    private void createOrUpdateStatus(final Orbit orbit) {
        final OrbitStatusBuilder builder =
                orbit.getStatus() != null ? orbit.getStatus().edit() : new OrbitStatusBuilder();
        final Instant now = Instant.now();

        builder.withPhase(OrbitStatus.Phase.RUNNING);

        if (!builder.hasStartTime()) {
            builder.withStartTime(DateTimeFormatter.ISO_INSTANT.format(now));
        }

        if (!builder.hasExpirationTime()
                && orbit.getSpec().getLifetime() != null
                && !orbit.getSpec().getLifetime().isEmpty()) {
            final Duration dur = Duration.parse(orbit.getSpec().getLifetime());
            builder.withExpirationTime(DateTimeFormatter.ISO_INSTANT.format(dur.addTo(now)));
        }

        builder.addNewCondition()
                .withType("Ready")
                .withStatus("True")
                .withLastTransitionTime(DateTimeFormatter.ISO_INSTANT.format(now))
                .endCondition();

        orbit.setStatus(builder.build());
    }
}
