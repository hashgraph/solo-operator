// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.capsuleconfigmap;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;

@ControllerConfiguration(dependents = {@Dependent(type = CapsuleConfigMap.class)})
public class CapsuleConfigMapReconciler implements Reconciler<ConsensusCapsule> {
    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            final ConsensusCapsule consensusCapsule, final Context<ConsensusCapsule> context) {
        return UpdateControl.noUpdate();
    }
}
