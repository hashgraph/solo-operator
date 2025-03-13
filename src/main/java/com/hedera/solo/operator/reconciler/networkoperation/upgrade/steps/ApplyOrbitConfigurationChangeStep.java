// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
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
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * ApplyOrbitConfigurationChangeStep is a step that applies the orbit level configuration changes
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
    public static final String CONDITION_TYPE = "OrbitUpdated";
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully updated orbit '%s'";
    public static final String FAIL_STATUS_MESSAGE = "Failed to update orbit '%s': %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            log.debugf(
                    "Skipping orbit change for network operation: %s/%s [ Phase: %s, Done: %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    status.getPhase(),
                    isDone(status));
            return status;
        }

        KubernetesClient client = context.getClient();
        NowFrozenOrbit nowFrozen = new NowFrozenOrbit(resource);

        log.infof(
                "Applying orbit change for network operation: %s/%s",
                resource.getMetadata().getNamespace(), resource.getMetadata().getName());

        client.resources(Orbit.class).withName(resource.getSpec().getOrbit()).edit(nowFrozen::update);

        log.infof("Successfully updated orbit: %s", resource.getMetadata().getName());

        return edit(status)
                .with(s -> s.addToConditions(new ConditionBuilder()
                        .withObservedGeneration(resource.getMetadata().getGeneration())
                        .withType(CONDITION_TYPE)
                        .withStatus(ConditionStatus.True)
                        .withLastTransitionTime(timestamp())
                        .withMessage(SUCCESS_STATUS_MESSAGE.formatted(
                                resource.getSpec().getOrbit()))
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
                "Failed to update orbit for network operation %s/%s: %s"
                        .formatted(
                                resource.getMetadata().getNamespace(),
                                resource.getMetadata().getName(),
                                resource.getStatus().getConsensusCapsules()));

        final String failMessage =
                FAIL_STATUS_MESSAGE.formatted(resource.getSpec().getOrbit(), ex.getMessage());
        return NetworkOperationStepUtil.prepareFailed(resource, status, failMessage, CONDITION_TYPE);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
