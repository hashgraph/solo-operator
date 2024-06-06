package com.hedera.solo.operator.reconciler.xp;

import com.hedera.solo.operator.v1alpha1.StatusUpdater;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.RetryInfo;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import io.javaoperatorsdk.operator.processing.event.source.informer.InformerEventSource;
import io.javaoperatorsdk.operator.processing.retry.GradualRetry;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Collections;
import java.util.Map;
import java.util.Set;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

//@FullJitterRetry
@ControllerConfiguration(dependents = {
        @Dependent(type = StatusUpdaterRegistry.class),
        @Dependent(type = StatusUpdaterRegistryNoSSA.class)
}
//, retry = MyRetry.class
)
public class StatusUpdaterReconciler implements /*EventSourceInitializer<StatusUpdater>,*/ Reconciler<StatusUpdater>,
        ErrorStatusHandler<StatusUpdater>
{
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public UpdateControl<StatusUpdater> reconcile(StatusUpdater statusUpdater, Context<StatusUpdater> context) throws Exception {
//        double count = statusUpdater.getStatus() == null ? 0 : statusUpdater.getStatus().getCount();
//        return UpdateControl.patchStatus(statusUpdater.edit().editStatus().withCount(count + 1).endStatus().build())
//                ;//.rescheduleAfter(Duration.ofSeconds(99999));
        return UpdateControl.noUpdate();
    }

//    @Override
//    public Map<String, EventSource> prepareEventSources(EventSourceContext<StatusUpdater> context) {
//        var es = new InformerEventSource<>(InformerConfiguration.from(ConfigMap.class, context)
////                .withPrimaryToSecondaryMapper(p ->
////                        p.getMetadata().getName().endsWith("3") ?
////                                Collections.emptySet() :
////                                Set.of(new ResourceID("status-updater", p.getMetadata().getNamespace())))
////                .withSecondaryToPrimaryMapper(s -> Collections.emptySet())
//                .build(), context);
//        var nossa = new InformerEventSource<>(InformerConfiguration.from(ConfigMap.class, context)
////                .withPrimaryToSecondaryMapper(p ->
////                        p.getMetadata().getName().endsWith("5") ?
////                                Collections.emptySet() :
////                                Set.of(new ResourceID("status-updater-no-ssa", p.getMetadata().getNamespace())))
////                .withSecondaryToPrimaryMapper(s -> Collections.emptySet())
//                .build(), context);
//        return Map.of("myEventSource", es
//                , "myEventSourceNoSSA", nossa
//        );
//    }

    @Override
    public ErrorStatusUpdateControl<StatusUpdater> updateErrorStatus(StatusUpdater statusUpdater, Context<StatusUpdater> context, Exception e) {
        log.info("Updating status update failed {} {} {}", statusUpdater.getMetadata().getName(), context.getRetryInfo().get().getAttemptCount(), context.getRetryInfo().get().isLastAttempt());
        return ErrorStatusUpdateControl.noStatusUpdate();
    }
}
