// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CreateClusterConfigStep;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CreatePublicKeysConfigStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.List;
import org.jboss.logging.Logger;

@ApplicationScoped
public class NetworkGenesisTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofSeconds(1);

    private final CustomResourceManager customResourceManager;

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Inject
    NetworkCheckProcessor deferredProcessor;

    @Inject
    public NetworkGenesisTask(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(
                new CreateClusterConfigStep(customResourceManager),
                new CreatePublicKeysConfigStep(customResourceManager));
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

    public static boolean isDone(NetworkOperation resource) {
        return CreateClusterConfigStep.isDone(resource.getStatus())
                && CreatePublicKeysConfigStep.isDone(resource.getStatus());
    }
}
