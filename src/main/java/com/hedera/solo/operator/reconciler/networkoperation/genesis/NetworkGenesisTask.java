// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.library.processor.RetryStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CheckConsensusNodeSecretsStep;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CreateClusterConfigStep;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CreatePublicKeysConfigStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.List;
import org.jboss.logging.Logger;

/**
 * The NetworkGenesisTask class is responsible for orchestrating the generation of a new Hedera network, including
 * creating cluster configuration and public keys.
 */
@ApplicationScoped
public class NetworkGenesisTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofSeconds(1);

    private final CustomResourceManager customResourceManager;

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final NetworkCheckProcessor deferredProcessor;

    private final NetworkGenesisTaskConfig genesisConfig;

    /**
     * Constructor for the NetworkGenesisTask class, injecting dependencies.
     *
     * @param customResourceManager Custom resource manager for Kubernetes resources.
     * @param deferredProcessor Processor for network checks.
     * @param genesisConfig Configuration settings for the network generation task.
     */
    @Inject
    public NetworkGenesisTask(
            CustomResourceManager customResourceManager,
            NetworkCheckProcessor deferredProcessor,
            NetworkGenesisTaskConfig genesisConfig) {
        this.customResourceManager = customResourceManager;
        this.deferredProcessor = deferredProcessor;
        this.genesisConfig = genesisConfig;
    }

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new CheckConsensusNodeSecretsStep(customResourceManager),
                new CreatePublicKeysConfigStep(customResourceManager),
                new RetryStep<>(
                        new CreateClusterConfigStep(customResourceManager),
                        Duration.ofSeconds(genesisConfig.retryInterval()),
                        genesisConfig.maxRetries()));
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
            log.infof("Network genesis is done, deferring to network check");
            return deferredProcessor;
        }

        return null;
    }

    /**
     * Checks if the network generation process is complete by verifying the completion of specific steps.
     *
     * @param resource The NetworkOperation resource to be checked.
     * @return True if the network generation is done, false otherwise
     */
    public static boolean isDone(NetworkOperation resource) {
        return CheckConsensusNodeSecretsStep.isDone(resource.getStatus())
                && CreateClusterConfigStep.isDone(resource.getStatus())
                && CreatePublicKeysConfigStep.isDone(resource.getStatus());
    }
}
