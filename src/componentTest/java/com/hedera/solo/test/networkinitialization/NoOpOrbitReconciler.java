package com.hedera.solo.test.networkinitialization;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

@ControllerConfiguration
public class NoOpOrbitReconciler implements Reconciler<Orbit> {
    @Override
    public UpdateControl<Orbit> reconcile(Orbit resource, Context<Orbit> context) {
        return UpdateControl.noUpdate();
    }
}
