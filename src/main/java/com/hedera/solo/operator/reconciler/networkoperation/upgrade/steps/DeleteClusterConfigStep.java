// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.time.Instant;
import java.util.Objects;
import org.jboss.logging.Logger;

public class DeleteClusterConfigStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ClusterConfigDeleted";
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
            return status;
        }

        try {
            final String namespace = resource.getMetadata().getNamespace();
            final ConfigMap existing = customResourceManager.configMapByName(namespace, Constants.CLUSTER_CONFIG_NAME);
            if (existing != null) {
                customResourceManager.getKubernetesClient().resource(existing).delete();
                log.info("Deleted cluster config: %s"
                        .formatted(existing.getMetadata().getName()));
                return edit(status)
                        .with(s -> s.addToConditions(new ConditionBuilder()
                                .withType(CONDITION_TYPE)
                                .withStatus(ConditionStatus.True)
                                .withLastTransitionTime(Instant.now().toString())
                                .withMessage("Successfully deleted cluster config: %s"
                                        .formatted(Constants.CLUSTER_CONFIG_NAME))
                                .build()))
                        .build();
            }

            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                            .withType(CONDITION_TYPE)
                            .withStatus(ConditionStatus.True)
                            .withLastTransitionTime(Instant.now().toString())
                            .withMessage("Nothing to delete. config '%s/%s' doesn't exist"
                                    .formatted(resource.getMetadata().getNamespace(), Constants.CLUSTER_CONFIG_NAME))
                            .build()))
                    .build();

        } catch (Exception ex) {
            log.error("Failed to delete cluster config", ex);
            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withType(CONDITION_TYPE)
                                    .withStatus(ConditionStatus.False)
                                    .withLastTransitionTime(Instant.now().toString())
                                    .withMessage("Failed to delete cluster config: %s".formatted(ex.getMessage()))
                                    .build())
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }
    }

    public static boolean isDone(NetworkOperationStatus status) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(
                        c -> c.getType().equals(CONDITION_TYPE) && c.getStatus().equals(ConditionStatus.True));
    }
}
