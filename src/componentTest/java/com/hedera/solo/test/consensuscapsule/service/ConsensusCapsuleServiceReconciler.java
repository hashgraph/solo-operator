// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.consensuscapsule.service;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleService;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Workflow;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;

/**
 * The ConsensusCapsuleServiceReconciler class is responsible for reconciling ConsensusCapsule instances by managing
 * their dependent CapsuleService resources.
 */
@Workflow(dependents = {@Dependent(type = CapsuleService.class)})
@ControllerConfiguration
public class ConsensusCapsuleServiceReconciler implements Reconciler<ConsensusCapsule> {
    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            ConsensusCapsule consensusCapsule, Context<ConsensusCapsule> context) throws Exception {
        return UpdateControl.noUpdate();
    }
}
