package com.hedera.solo.operator.reconciler.xp;

import com.hedera.solo.operator.v1alpha1.StatusUpdater;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import io.javaoperatorsdk.operator.processing.event.source.informer.InformerEventSource;
import java.time.Duration;
import java.util.Map;
import java.util.Set;

@ControllerConfiguration(dependents = {
        @Dependent(type = StatusUpdaterRegistry.class),
        @Dependent(type = StatusUpdaterRegistryNoSSA.class)
})
public class StatusUpdaterReconciler implements EventSourceInitializer<StatusUpdater>, Reconciler<StatusUpdater> {
    @Override
    public UpdateControl<StatusUpdater> reconcile(StatusUpdater statusUpdater, Context<StatusUpdater> context) throws Exception {
        double count = statusUpdater.getStatus() == null ? 0 : statusUpdater.getStatus().getCount();
        return UpdateControl.patchStatus(statusUpdater.edit().editStatus().withCount(count + 1).endStatus().build())
                ;//.rescheduleAfter(Duration.ofSeconds(99999));
    }

    @Override
    public Map<String, EventSource> prepareEventSources(EventSourceContext<StatusUpdater> context) {
        var es = new InformerEventSource<>(InformerConfiguration.from(ConfigMap.class, context)
                .withPrimaryToSecondaryMapper(p -> Set.of(new ResourceID("status-updater", p.getMetadata().getNamespace())))
                .build(), context);
        var nossa = new InformerEventSource<>(InformerConfiguration.from(ConfigMap.class, context)
                .withPrimaryToSecondaryMapper(p -> Set.of(new ResourceID("status-updater-no-ssa", p.getMetadata().getNamespace())))
                .build(), context);
        return Map.of("myEventSource", es
                , "myEventSourceNoSSA", nossa
        );
    }
}
