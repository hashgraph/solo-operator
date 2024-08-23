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

package com.hedera.solo.operator.reconciler.capsule.haproxy;

import com.hedera.solo.operator.library.generic.Compare;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyConfigMap;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyDeployment;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyPod;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxySecret;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyService;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import jakarta.inject.Inject;
import java.util.Objects;
import org.jboss.logging.Logger;

@ControllerConfiguration(
        dependents = {
            @Dependent(type = HAProxyConfigMap.class),
            @Dependent(type = HAProxySecret.class, name = "HAProxySecret"),
            @Dependent(type = HAProxyDeployment.class, dependsOn = "HAProxySecret"),
            @Dependent(type = HAProxyPod.class),
            @Dependent(type = HAProxyService.class),
        })
public class HAProxyCapsuleReconciler implements Reconciler<HAProxyCapsule> {
    private final Logger log;

    @Inject
    public HAProxyCapsuleReconciler(final Logger log) {
        this.log = log;
    }

    @Override
    public UpdateControl<HAProxyCapsule> reconcile(HAProxyCapsule resource, Context<HAProxyCapsule> context)
            throws Exception {

        HAProxyCapsule desired = resource.edit()
                .editStatus()
                .withObservedGeneration(resource.getMetadata().getGeneration())
                .withPhase(this.getDesiredPhase(context))
                .endStatus()
                .build();

        if (!requireUpdate(resource, desired)) {
            log.infof(
                    "HAProxy Capsule Reconciler - No update required [ phase = '%s', observedGeneration = '%s' ]",
                    desired.getStatus().getPhase(), desired.getStatus().getObservedGeneration());

            return UpdateControl.noUpdate();
        }

        log.infof(
                "HAProxy Capsule Reconciler - Reconciling [ phase = '%s' observedGeneration = '%d' ]",
                desired.getStatus().getPhase(), desired.getStatus().getObservedGeneration());

        return UpdateControl.patchStatus(desired);
    }

    private HAProxyCapsuleStatus.Phase getDesiredPhase(Context<HAProxyCapsule> context) {
        HAProxyCapsuleStatus.Phase phase = HAProxyCapsuleStatus.Phase.Unknown;
        Pod[] pods = context.getSecondaryResources(Pod.class).toArray(Pod[]::new);
        if (1 == pods.length) {
            phase = HAProxyCapsuleStatus.Phase.valueOf(pods[0].getStatus().getPhase());
        }

        return phase;
    }

    private boolean requireUpdate(HAProxyCapsule current, HAProxyCapsule desired) {
        return !Compare.allMatch(
                current,
                desired,
                o -> !Objects.isNull(o.getStatus()) ? o.getStatus().getObservedGeneration() : -1,
                o -> !Objects.isNull(o.getStatus()) ? o.getStatus().getPhase() : HAProxyCapsuleStatus.Phase.Unknown);
    }
}
