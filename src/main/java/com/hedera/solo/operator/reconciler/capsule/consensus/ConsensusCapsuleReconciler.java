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

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleDeployment;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleRoleBinding;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleSecret;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleService;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleServiceAccount;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import jakarta.inject.Inject;
import java.util.Objects;
import java.util.function.Function;
import org.jboss.logging.Logger;

@ControllerConfiguration(
        name = "consensus-capsule",
        dependents = {
            @Dependent(name = "sa", type = CapsuleServiceAccount.class),
            @Dependent(type = CapsuleRoleBinding.class, dependsOn = "sa"),
            @Dependent(type = CapsuleDeployment.class),
            @Dependent(type = CapsuleSecret.class),
            @Dependent(type = CapsuleService.class),
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
        try {
            log.infof(
                    "Consensus Capsule - Starting Reconciliation [ name = '%s' ]",
                    capsule.getMetadata().getName());

            ConsensusCapsuleStatus.Phase phase = 1
                            == context.getSecondaryResource(Deployment.class)
                                    .map(Deployment::getStatus)
                                    .map(DeploymentStatus::getAvailableReplicas)
                                    .orElse(0)
                    ? ConsensusCapsuleStatus.Phase.Running
                    : ConsensusCapsuleStatus.Phase.Unknown;

            ConsensusCapsule desired = capsule.edit()
                    .editStatus()
                    .withObservedGeneration(capsule.getMetadata().getGeneration())
                    .withPhase(phase)
                    .endStatus()
                    .build();

            try {
                if (allMatch(capsule, desired, o -> o.getStatus().getObservedGeneration(), o -> o.getStatus()
                        .getPhase())) {
                    return UpdateControl.noUpdate();
                }
            } catch (NullPointerException e) {
            }
            return UpdateControl.patchStatus(desired);
        } finally {
            log.infof(
                    "Consensus Capsule - Finished Reconciliation [ name = '%s' ]",
                    capsule.getMetadata().getName());
        }
    }

    @SafeVarargs
    public static final <T> boolean allMatch(final T a, final T b, final Function<T, ?>... mappers) {
        for (Function<T, ?> mapper : mappers) {
            if (!Objects.equals(mapper.apply(a), mapper.apply(b))) {
                return false;
            }
        }
        return true;
    }

    @Override
    public ErrorStatusUpdateControl<ConsensusCapsule> updateErrorStatus(
            final ConsensusCapsule capsule, final Context<ConsensusCapsule> context, final Exception e) {
        return ErrorStatusUpdateControl.noStatusUpdate();
    }
}
