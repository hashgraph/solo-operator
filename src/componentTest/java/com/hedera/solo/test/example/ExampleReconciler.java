// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.example;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;

@ControllerConfiguration(dependents = {@Dependent(type = ExampleConfigMap.class)})
public class ExampleReconciler implements Reconciler<Example> {
    @Override
    public UpdateControl<Example> reconcile(Example example, Context<Example> context) throws Exception {

        return UpdateControl.patchStatus(example.edit()
                .editStatus()
                .withObservedGeneration(example.getMetadata().getGeneration())
                .endStatus()
                .build());
    }
}
