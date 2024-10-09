package com.hedera.solo.test.networkinitialization;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

@ControllerConfiguration
public class NetworkInitializationTestReconciler extends NetworkInitializationReconciler {
    @Override
    public UpdateControl<NetworkInitialization> reconcile(NetworkInitialization resource, Context<NetworkInitialization> context) throws Exception {
        // todo: use label or annotation instead of name
        if (null != resource.getStatus() && "true".equals(resource.getMetadata().getLabels().get("no-update"))) {
            return UpdateControl.noUpdate();
        }
        return super.reconcile(resource, context);
    }
}
