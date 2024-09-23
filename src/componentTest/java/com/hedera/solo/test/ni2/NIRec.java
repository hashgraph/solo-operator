package com.hedera.solo.test.ni2;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
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
import java.util.concurrent.atomic.AtomicLong;

@ControllerConfiguration
public class NIRec implements Reconciler<NetworkInitialization>/*, EventSourceInitializer<NetworkInitialization>*/ {
    AtomicLong counter = new AtomicLong();
//    SeqDR seqDR = new SeqDR();
//    Workflow<NetworkInitialization> workflow = new WorkflowBuilder<NetworkInitialization>()
//            .addDependentResource(seqDR)
//            .build();

    @Override
    public UpdateControl<NetworkInitialization> reconcile(NetworkInitialization resource, Context<NetworkInitialization> context) throws Exception {
        // just add seq num
        if (null == resource.getStatus()) {
            return UpdateControl.patchStatus(edit(resource)
                    .with(builder -> builder
                            .editStatus()
                            .withSequenceNumber(counter.incrementAndGet())
                            .withPhase(NetworkInitializationStatus.Phase.Pending))
                    .build());
        }
//        workflow.reconcile(resource, context);
        return UpdateControl.noUpdate();
    }

//    @Override
//    public Map<String, EventSource> prepareEventSources(final EventSourceContext<NetworkInitialization> context) {
//        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
//    }
}
