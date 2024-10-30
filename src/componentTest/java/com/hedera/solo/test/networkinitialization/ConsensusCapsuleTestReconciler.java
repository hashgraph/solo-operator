// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkinitialization;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

@ControllerConfiguration
public class ConsensusCapsuleTestReconciler implements Reconciler<ConsensusCapsule> {
    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            final ConsensusCapsule resource, final Context<ConsensusCapsule> context) {
        return UpdateControl.patchStatus(edit(resource)
                .with(status -> status.editStatus()
                        .withPhase(
                                resource.getSpec().getEligibleForDeployment()
                                        ? ConsensusCapsuleStatus.Phase.Running
                                        : ConsensusCapsuleStatus.Phase.Pending))
                .build());
    }
}
