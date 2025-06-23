// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.render.NowFrozenOrbit;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * ApplyOrbitConfigurationChangeStep is a step that applies the orbit level configuration changes for a network
 * operation.
 *
 * <p>It rolls out config file bundle as specified in "configurationFiles" field and patch the orbit CR by making direct
 * API call to Kubernetes API server. The effect of the orbit CR changes will be the following that
 * {@link NetworkCheckProcessor}] will check:
 *
 * <ol>
 *   <li>Orbit reconciler creates config-maps: settings.txt, application.properties, api-permission.properties, etc.
 *   <li>Network-operation reconciler creates config.txt.
 * </ol>
 *
 * <p>If this step is re-invoked, the changes in orbit CR should be idempotent.
 */
public class ApplyOrbitConfigurationChangeStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE = new StepConditionType("ResourceUpdated", "Orbit");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully updated orbit '%s'";
    public static final String FAIL_STATUS_MESSAGE = "Failed to update orbit '%s': %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        try {
            Objects.requireNonNull(resource, "NetworkOperation is required");
            Objects.requireNonNull(resource.getSpec(), "NetworkOperationSpec is required");
            Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "NetworkUpgrade is required");
            Objects.requireNonNull(status, "NetworkOperationStatus is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> is required");

            KubernetesClient client = context.getClient();
            NowFrozenOrbit nowFrozen = new NowFrozenOrbit(resource);

            log.infof(
                    "Applying orbit change for network operation: %s/%s [ %s]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            client.resources(Orbit.class)
                    .withName(resource.getSpec().getOrbit())
                    .edit(nowFrozen::update);

            log.infof(
                    "Successfully updated orbit: %s [ %s ] ",
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            final Condition condition = new ConditionBuilder()
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withType(CONDITION_TYPE.type())
                    .withReason(CONDITION_TYPE.reason())
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(timestamp())
                    .withMessage(
                            SUCCESS_STATUS_MESSAGE.formatted(resource.getSpec().getOrbit()))
                    .build();
            return NetworkOperationStepUtil.prepareStatus(status, List.of(condition));
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    /**
     * Checks if the step is done based on the current status of the network operation.
     *
     * @param status The current status of the NetworkOperation.
     * @return True if the step is done, false otherwise
     */
    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
