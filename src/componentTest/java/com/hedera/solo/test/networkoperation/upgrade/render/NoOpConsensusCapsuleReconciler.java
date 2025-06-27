// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

/**
 * The NoOpConsensusCapsuleReconciler class is a no-operation reconciler for ConsensusCapsule resources. It does not
 * perform any updates or modifications on the ConsensusCapsule resource during reconciliation.
 */
@ControllerConfiguration
public class NoOpConsensusCapsuleReconciler implements Reconciler<ConsensusCapsule> {
    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            ConsensusCapsule consensusCapsule, Context<ConsensusCapsule> context) throws Exception {
        return UpdateControl.noUpdate();
    }
}
