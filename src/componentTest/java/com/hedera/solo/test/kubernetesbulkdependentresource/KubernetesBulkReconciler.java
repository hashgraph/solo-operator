package com.hedera.solo.test.kubernetesbulkdependentresource;

import static com.hedera.solo.operator.library.builder.BuilderHelper.edit;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

public class KubernetesBulkReconcilerBase implements Reconciler<Example> {
    @Override
    public UpdateControl<Example> reconcile(Example resource, Context<Example> context) throws Exception {
        return UpdateControl.patchStatus(edit(resource)
                .with(builder -> builder
                        .editStatus()
                        .withObservedGeneration(resource.getMetadata().getGeneration()))
                .build());
    }
}
