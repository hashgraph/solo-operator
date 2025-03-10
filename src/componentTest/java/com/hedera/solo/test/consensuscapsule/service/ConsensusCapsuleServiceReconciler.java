// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.consensuscapsule.service;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleService;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;

@ControllerConfiguration(dependents = {@Dependent(type = CapsuleService.class)})
public class ConsensusCapsuleServiceReconciler implements Reconciler<ConsensusCapsule> {
    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            ConsensusCapsule consensusCapsule, Context<ConsensusCapsule> context) throws Exception {
        return UpdateControl.noUpdate();
    }
}
