package com.hedera.solo.test.readonlybulkdependentresource;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;

@ControllerConfiguration(dependents = @Dependent(type = ReadOnlyBulkDependentResource.class))
public class ReadOnlyBulkDependentResourceReconciler implements Reconciler<Example> {
    @Override
    public UpdateControl<Example> reconcile(Example resource, Context<Example> context) throws Exception {
        return UpdateControl.patchStatus(resource.edit().editStatus().withObservedGeneration(resource.getMetadata().getGeneration()).endStatus().build());
    }
}
