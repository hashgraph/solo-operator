// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.kubernetesbulkdependentresource;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.util.Map;

@ControllerConfiguration
public class KubernetesBulkDependentResourceReconciler implements EventSourceInitializer<Example>, Reconciler<Example> {
    private final Workflow<Example> workflow;

    public KubernetesBulkDependentResourceReconciler(final DependentResource<ConfigMap, Example> dependentResource) {
        workflow = new WorkflowBuilder<Example>()
                .addDependentResource(dependentResource)
                .build();
    }

    @Override
    public UpdateControl<Example> reconcile(final Example resource, final Context<Example> context) throws Exception {
        workflow.reconcile(resource, context);
        return UpdateControl.patchStatus(edit(resource)
                .with(status -> status.editStatus()
                        .withMessage(String.valueOf(
                                context.getSecondaryResources(ConfigMap.class).size()))
                        .withObservedGeneration(resource.getMetadata().getGeneration()))
                .build());
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<Example> context) {
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
