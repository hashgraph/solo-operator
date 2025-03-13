// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

public class DeleteClusterConfigStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ClusterConfigDeleted";
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully deleted '%s'";
    public static final String FAIL_STATUS_MESSAGE = "Failed to delete '%s':'%s'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    public DeleteClusterConfigStep(CustomResourceManager customResourceManager) {
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
                    "Skipping deletion of '%s' for network operation: %s/%s [ Phase: %s, Done: %s ]",
                    Constants.CLUSTER_CONFIG_NAME,
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    status.getPhase(),
                    isDone(status));
            return status;
        }

        final String namespace = resource.getMetadata().getNamespace();
        final ConfigMap existing = customResourceManager.configMapByName(namespace, Constants.CLUSTER_CONFIG_NAME);
        if (existing != null) {
            log.debugf(
                    "Deleting '%s' for network operation: %s/%s",
                    Constants.CLUSTER_CONFIG_NAME,
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());

            customResourceManager.getKubernetesClient().resource(existing).delete();

            log.debugf(
                    "Successfully deleted '%s' for network operation: %s/%s",
                    Constants.CLUSTER_CONFIG_NAME,
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());
        } else {
            log.warnf(
                    "'%s/%s' doesn't exist. Nothing to delete, continuing to next step...",
                    resource.getMetadata().getNamespace(), Constants.CLUSTER_CONFIG_NAME);
        }

        return edit(status)
                .with(s -> s.addToConditions(new ConditionBuilder()
                        .withObservedGeneration(resource.getMetadata().getGeneration())
                        .withType(CONDITION_TYPE)
                        .withStatus(ConditionStatus.True)
                        .withLastTransitionTime(timestamp())
                        .withMessage(SUCCESS_STATUS_MESSAGE.formatted(Constants.CLUSTER_CONFIG_NAME))
                        .build()))
                .build();
    }

    @Override
    public NetworkOperationStatus handleError(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context, Exception ex) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperation status is required");
        Objects.requireNonNull(ex, "Exception is required");

        log.errorf(
                ex,
                "Failed to delete cluster config: %s/%s",
                resource.getMetadata().getNamespace(),
                Constants.CLUSTER_CONFIG_NAME);

        final String failMessage = FAIL_STATUS_MESSAGE.formatted(Constants.CLUSTER_CONFIG_NAME, ex.getMessage());
        return NetworkOperationStepUtil.prepareFailed(resource, status, failMessage, CONDITION_TYPE);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
