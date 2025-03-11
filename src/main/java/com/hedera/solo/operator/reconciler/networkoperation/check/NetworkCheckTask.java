// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.steps.CheckConsensusNodeReadyStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.List;

/**
 * NetworkCheckTask checks that network is in valid state
 *
 * <p>A valid network state check be defined by the following:
 *
 * <ol>
 *   <li>Verify that actual version updates are applied to the pods based on the CR and mode of operation (e.g.
 *       NetworkGenesis, NetworkUpgrade etc.)
 *   <li>Verify that all configuration are applied correctly based on the CR and mode of operation
 *   <li>consensus nodes are in ACTIVE state and processing transactions.
 *   <li>stream uploaders are running and uploading files
 *   <li>Other auxiliary components (if deployed) are running (no need for phase-1 (MVP) release)
 *       <ol>
 *         <li>HAProxy
 *         <li>EnvoyProxy
 *         <li>JSON-RPC relay
 *         <li>Mirror-node components
 *       </ol>
 * </ol>
 *
 * <p>It updates {@link NetworkOperationStatus} with condition describing the status of the network
 */
@ApplicationScoped
public class NetworkCheckTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofSeconds(1);

    private final CustomResourceManager customResourceManager;

    @Inject
    public NetworkCheckTask(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                // new CheckOrbitConfigurationStep(customResourceManager),
                // new CheckConsensusPodConfigurationStep(customResourceManager),
                new CheckConsensusNodeReadyStep(customResourceManager));
    }

    @Override
    protected Duration computeReconcileDelay(NetworkOperation resource, NetworkOperationStatus status)
            throws ProcessorException {
        return RECONCILE_DELAY;
    }

    @Override
    protected Processor<?, ?, ?> computeDeferredProcessor(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        return null; // it does not have any deferred processor
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return CheckConsensusNodeReadyStep.isDone(status);
    }
}
