// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * CheckConsensusPodConfigurationStep checks that consensus pod configuration are rolled out
 *
 * <p>It needs to inspect the CR and performs the necessary checks to ensure that the consensus nodes are running with
 * correct version and configuration files.
 */
public class CheckConsensusPodConfigurationStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ConsensusPodUpdated";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    public CheckConsensusPodConfigurationStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            log.debugf(
                    "Skipping verification of 'consensus-pod' configuration changes for network operation: %s/%s [ Phase: %s, Done: %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    status.getPhase(),
                    isDone(status));
            return status;
        }

        return status; // FIXME implement the logic
    }

    @Override
    public NetworkOperationStatus handleError(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context, Exception ex) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperation status is required");
        Objects.requireNonNull(ex, "Exception is required");

        log.error("Failed to check consensus pod configuration", ex);
        return NetworkOperationStepUtil.prepareFailed(resource, status, ex.getMessage(), CONDITION_TYPE);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
