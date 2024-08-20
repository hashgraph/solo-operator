/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
    public UpdateControl<Example> reconcile(Example resource, Context<Example> context) throws Exception {
        workflow.reconcile(resource, context);
        return UpdateControl.patchStatus(edit(resource)
                .with(builder -> builder.editStatus()
                        .withMessage(String.valueOf(
                                context.getSecondaryResources(ConfigMap.class).size()))
                        .withObservedGeneration(resource.getMetadata().getGeneration()))
                .build());
    }

    @Override
    public Map<String, EventSource> prepareEventSources(EventSourceContext<Example> context) {
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
