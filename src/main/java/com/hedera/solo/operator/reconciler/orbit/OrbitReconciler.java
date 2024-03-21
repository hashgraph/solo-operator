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

import com.hedera.solo.operator.model.orbit.Orbit;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import jakarta.inject.Inject;
import org.jboss.logging.Logger;

@ControllerConfiguration(name = "orbit")
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
            return UpdateControl.updateStatus(orbit);
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
        return ErrorStatusUpdateControl.updateStatus(orbit);
    }
}
