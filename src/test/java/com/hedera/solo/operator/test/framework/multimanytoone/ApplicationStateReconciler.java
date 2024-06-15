package com.hedera.solo.operator.test.framework.multimanytoone;

import com.hedera.solo.operator.model.v1alpha1.ApplicationState;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

@ControllerConfiguration
public class ApplicationStateReconciler implements Reconciler<ApplicationState> {
    @Override
    public UpdateControl<ApplicationState> reconcile(ApplicationState resource, Context<ApplicationState> context) throws Exception {
        return UpdateControl.noUpdate();
    }
}
