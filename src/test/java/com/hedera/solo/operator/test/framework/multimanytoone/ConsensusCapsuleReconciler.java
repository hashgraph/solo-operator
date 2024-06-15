package com.hedera.solo.operator.test.framework.multimanytoone;

import com.hedera.solo.operator.model.v1alpha1.ApplicationState;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import io.javaoperatorsdk.operator.processing.event.source.informer.InformerEventSource;
import java.util.Collections;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.atomic.AtomicInteger;

@ControllerConfiguration(dependents = {
        @Dependent(type = ConsensusCapsuleApplicationState.class, useEventSourceWithName = "xyz")
})
public class ConsensusCapsuleReconciler implements
        ErrorStatusHandler<ConsensusCapsule>,
        EventSourceInitializer<ApplicationState>,
        Reconciler<ConsensusCapsule>
{
    @Override
    public UpdateControl<ConsensusCapsule> reconcile(ConsensusCapsule resource, Context<ConsensusCapsule> context) throws Exception {
        return UpdateControl.noUpdate();
    }

    @Override
    public Map<String, EventSource> prepareEventSources(EventSourceContext<ApplicationState> context) {
        var configMapEventSource =
                new InformerEventSource<>(InformerConfiguration.from(ApplicationState.class, context)
                        .withPrimaryToSecondaryMapper(p -> Set.of(new ResourceID("application-state", p.getMetadata().getNamespace())))
                        .withSecondaryToPrimaryMapper(r ->
                                Collections.emptySet()
//                                r.getMetadata().getOwnerReferences().stream()
//                                .filter(x -> x.getKind().equals("ConsensusCapsule"))
//                                .map(x -> new ResourceID(x.getName(), r.getMetadata().getNamespace())).collect(Collectors.toSet())
                        )
                        .build(), context);
        return Map.of("xyz", configMapEventSource);
    }

    public static AtomicInteger counter = new AtomicInteger();
    @Override
    public ErrorStatusUpdateControl<ConsensusCapsule> updateErrorStatus(ConsensusCapsule resource, Context<ConsensusCapsule> context, Exception e) {
        counter.incrementAndGet();
        return ErrorStatusUpdateControl.noStatusUpdate();
    }
}
