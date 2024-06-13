package com.hedera.solo.operator.test.unit;

import com.hedera.solo.operator.model.v1alpha1.StatusUpdater;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusHandler;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Dependent;
import java.lang.invoke.MethodHandles;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@ControllerConfiguration(dependents = {
        @Dependent(type = StatusUpdaterRegistry.class),
        @Dependent(type = StatusUpdaterRegistryNoSSA.class)
})
public class StatusUpdaterReconciler implements /*EventSourceInitializer<StatusUpdater>,*/ Reconciler<StatusUpdater>,
        ErrorStatusHandler<StatusUpdater>
{
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public UpdateControl<StatusUpdater> reconcile(StatusUpdater statusUpdater, Context<StatusUpdater> context) throws Exception {
        return UpdateControl.noUpdate();
    }

    @Override
    public ErrorStatusUpdateControl<StatusUpdater> updateErrorStatus(StatusUpdater statusUpdater, Context<StatusUpdater> context, Exception e) {
        return ErrorStatusUpdateControl.noStatusUpdate();
    }
}
