// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.library.processor.RetryStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CreateClusterConfigStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyConsensusPodChangeStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyOrbitConfigurationChangeStep;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.DeleteClusterConfigStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.List;
import org.jboss.logging.Logger;

/**
 * NowFrozenTask executes the network upgrade steps in response to the "now-frozen.mf" marker file
 *
 * <p>It performs the following action steps:
 *
 * <ol>
 *   <li>Reverify the consensus node and stream-uploader changesets and image sources. Check reverification steps of
 *       {@link FreezeScheduledTask}.
 *   <li>Rollout config file bundle as specified in "configurationFiles" field (patch the orbit CR).
 *       {@link NetworkCheckProcessor}] will check the following:
 *       <ol>
 *         <li>Orbit reconciler creates config-maps: settings.txt, application.properties, api-permission.properties,
 *             etc.
 *         <li>Network-operation reconciler creates config.txt.
 *       </ol>
 *   <li>Single atomic patch to consensus-capsule CR using the following changesets. It will be checked by
 *       {@link NetworkCheckProcessor} when deferred.
 *       <ol>
 *         <li>Apply consensus-node changeset (image source, nodeProperties and logging).
 *         <li>Apply streamUploader changesets (e.g., recordStreamUploader, backUploader, etc.).
 *       </ol>
 *   <li>Add a condition to the status field explaining the execution status.
 * </ol>
 *
 * <p>In the {@link NowFrozenTask#computeDeferredProcessor} method, check if it has already patched the CRs and return
 * the {@link NetworkCheckProcessor} to ensure the network is in a valid state.
 */
@ApplicationScoped
public class NowFrozenTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofSeconds(1);

    private final CustomResourceManager customResourceManager;
    private final NetworkCheckProcessor deferredProcessor;

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    /**
     * Constructor for NowFrozenTask.
     *
     * @param customResourceManager The {@link CustomResourceManager} to manage custom resources.
     * @param deferredProcessor The {@link NetworkCheckProcessor} to be executed after the current steps.
     */
    @Inject
    public NowFrozenTask(CustomResourceManager customResourceManager, NetworkCheckProcessor deferredProcessor) {
        this.customResourceManager = customResourceManager;
        this.deferredProcessor = deferredProcessor;
    }

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new DeleteClusterConfigStep(customResourceManager),
                new RetryStep<>(new ApplyConsensusPodChangeStep(), Duration.ofSeconds(3), 10),
                new ApplyOrbitConfigurationChangeStep(),
                new RetryStep<>(new CreateClusterConfigStep(customResourceManager), Duration.ofSeconds(2), 150));
    }

    @Override
    protected Duration computeReconcileDelay(NetworkOperation resource, NetworkOperationStatus status)
            throws ProcessorException {
        return RECONCILE_DELAY;
    }

    @Override
    protected Processor<?, ?, ?> computeDeferredProcessor(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        if (isDone(resource)) {
            log.infof("NowFrozen is done, deferring to network check");
            return deferredProcessor;
        }

        return null;
    }

    /**
     * Checks if the {@link NetworkOperation} has completed all its upgrade steps.
     *
     * @param resource The {@link NetworkOperation} resource being checked.
     * @return true if all upgrade steps are complete, otherwise false
     */
    public static boolean isDone(NetworkOperation resource) {
        return DeleteClusterConfigStep.isDone(resource.getStatus())
                && ApplyConsensusPodChangeStep.isDone(resource.getStatus())
                && ApplyOrbitConfigurationChangeStep.isDone(resource.getStatus())
                && CreateClusterConfigStep.isDone(resource.getStatus());
    }
}
