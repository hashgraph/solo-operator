package com.hedera.solo.test.updateonlybulkdependentresource;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import java.lang.invoke.MethodHandles;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@ControllerConfiguration(dependents = @Dependent(type = UpdateOnlyBulkDependentResource.class))
public class UpdateOnlyBulkDependentResourceReconciler implements Reconciler<Example> {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    @Override
    public UpdateControl<Example> reconcile(Example resource, Context<Example> context) throws Exception {
        try {
            log.info("Reconciling example {}", resource);
            return UpdateControl.patchStatus(resource.edit().editStatus().withObservedGeneration(resource.getMetadata().getGeneration()).endStatus().build());
        } finally {
            log.info("Finished reconciling example {}", resource);
        }
    }
}
