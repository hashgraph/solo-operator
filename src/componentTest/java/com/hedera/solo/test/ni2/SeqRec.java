package com.hedera.solo.test.ni2;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.util.Map;

@ControllerConfiguration
public class SeqRec implements Reconciler<NetworkInitialization>, EventSourceInitializer<NetworkInitialization> {
    SequenceDR sequenceDR = new SequenceDR();
    Workflow<NetworkInitialization> workflow = new WorkflowBuilder<NetworkInitialization>()
            .addDependentResource(sequenceDR)
            .build();

    @Override
    public UpdateControl<NetworkInitialization> reconcile(NetworkInitialization resource, Context<NetworkInitialization> context) throws Exception {
        workflow.reconcile(resource, context);
        return UpdateControl.noUpdate();
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkInitialization> context) {
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
