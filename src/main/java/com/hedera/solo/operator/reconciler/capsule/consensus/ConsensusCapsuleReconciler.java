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

package com.hedera.solo.operator.reconciler.capsule.consensus;

import com.hedera.solo.operator.library.generic.Compare;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleDeployment;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsulePod;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleRoleBinding;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleSecret;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleService;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleServiceAccount;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import jakarta.inject.Inject;
import java.util.Objects;
import org.jboss.logging.Logger;

@ControllerConfiguration(
        name = "consensus-capsule",
        dependents = {
            @Dependent(name = "sa", type = CapsuleServiceAccount.class),
            @Dependent(type = CapsuleRoleBinding.class, dependsOn = "sa"),
            @Dependent(name = "capsule-secret", type = CapsuleSecret.class),
            @Dependent(type = CapsuleDeployment.class, dependsOn = "capsule-secret"),
            @Dependent(type = CapsuleService.class),
            @Dependent(type = CapsulePod.class)
        })
public class ConsensusCapsuleReconciler implements Reconciler<ConsensusCapsule>, ErrorStatusHandler<ConsensusCapsule> {

    private final Logger log;

    @Inject
    public ConsensusCapsuleReconciler(final Logger log) {
        this.log = log;
    }

    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context) throws Exception {

        ConsensusCapsule desired = capsule.edit()
                .editStatus()
                .withObservedGeneration(capsule.getMetadata().getGeneration())
                .withPhase(this.getDesiredPhase(context))
                .endStatus()
                .build();

        if (!requireUpdate(capsule, desired)) {
            log.infof(
                    "Consensus Capsule Reconciler - No update required [ phase = '%s', observedGeneration = '%s' ]",
                    desired.getStatus().getPhase(), desired.getStatus().getObservedGeneration());

            return UpdateControl.noUpdate();
        }

        log.infof(
                "Consensus Capsule Reconciler - Reconciling [ phase = '%s' observedGeneration = '%d' ]",
                desired.getStatus().getPhase(), desired.getStatus().getObservedGeneration());

        return UpdateControl.patchStatus(desired);
    }

    private ConsensusCapsuleStatus.Phase getDesiredPhase(Context<ConsensusCapsule> context) {
        ConsensusCapsuleStatus.Phase phase = ConsensusCapsuleStatus.Phase.Unknown;
        Pod[] pods = context.getSecondaryResources(Pod.class).toArray(Pod[]::new);
        if (1 == pods.length) {
            phase = ConsensusCapsuleStatus.Phase.valueOf(pods[0].getStatus().getPhase());
        }
        return phase;
    }

    private boolean requireUpdate(ConsensusCapsule current, ConsensusCapsule desired) {
        return !Compare.allMatch(
                current,
                desired,
                o -> !Objects.isNull(o.getStatus()) ? o.getStatus().getObservedGeneration() : 1,
                o -> !Objects.isNull(o.getStatus()) ? o.getStatus().getPhase() : HAProxyCapsuleStatus.Phase.Unknown);
    }

    @Override
    public ErrorStatusUpdateControl<ConsensusCapsule> updateErrorStatus(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context, final Exception e) {
        return ErrorStatusUpdateControl.noStatusUpdate();
    }
}
