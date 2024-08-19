package com.hedera.solo.test.labelselector;


import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;

@ControllerConfiguration(
        labelSelector = "test-label-selector",
        dependents = {
                @Dependent(type = LabelSelectorResource.class),
//                @Dependent(type = NormalResource.class),
        })
public class LabelSelectorReconciler implements Reconciler<Example> {
    @Override
    public UpdateControl<Example> reconcile(Example resource, Context<Example> context) throws Exception {
        return UpdateControl.patchStatus(edit(resource)
                .with(builder -> builder
                        .editStatus()
                        .withObservedGeneration(resource.getMetadata().getGeneration()))
                .build());
    }
}
