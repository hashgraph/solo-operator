package com.hedera.solo.test.ni2;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSequence;
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
public class SeqRec implements Reconciler<NetworkInitializationSequence>, EventSourceInitializer<NetworkInitializationSequence> {
    NIDR nidr = new NIDR();
    Workflow<NetworkInitializationSequence> workflow = new WorkflowBuilder<NetworkInitializationSequence>()
            .addDependentResource(nidr)
            .build();

    @Override
    public UpdateControl<NetworkInitializationSequence> reconcile(NetworkInitializationSequence resource, Context<NetworkInitializationSequence> context) throws Exception {
        workflow.reconcile(resource, context);
        // maybe reschedule to run again
        if (context.getSecondaryResource(NetworkInitialization.class).isPresent()) {
            return UpdateControl.<NetworkInitializationSequence>noUpdate().rescheduleAfter(0);
        }
        return UpdateControl.noUpdate();
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkInitializationSequence> context) {
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
