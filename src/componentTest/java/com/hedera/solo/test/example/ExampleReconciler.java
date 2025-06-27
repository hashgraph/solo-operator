// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.example;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Workflow;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;

/**
 * The ExampleReconciler class is responsible for reconciling the Example resource in a Kubernetes cluster using the
 * Java Operators SDK.
 */
@Workflow(dependents = {@Dependent(type = ExampleConfigMap.class)})
@ControllerConfiguration
public class ExampleReconciler implements Reconciler<Example> {
    @Override
    public UpdateControl<Example> reconcile(Example example, Context<Example> context) throws Exception {

        return UpdateControl.patchStatus(edit(new Example())
                .with(metadata -> metadata.editMetadata()
                        .withName(example.getMetadata().getName())
                        .withNamespace(example.getMetadata().getNamespace()))
                .with(status -> status.editStatus()
                        .withObservedGeneration(example.getMetadata().getGeneration())
                        .endStatus())
                .build());
    }
}
