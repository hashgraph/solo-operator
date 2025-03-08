// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.render.NowFrozenOrbit;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.time.Instant;
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
            return status;
        }

        try {
            KubernetesClient client = context.getClient();
            NowFrozenOrbit nowFrozen = new NowFrozenOrbit(resource);
            client.resources(Orbit.class)
                    .withName(resource.getSpec().getOrbit())
                    .edit(nowFrozen::update);
        } catch (Exception ex) {
            log.errorf(
                    "Failed to update orbit for network operation %s/%s: %s"
                            .formatted(
                                    resource.getMetadata().getNamespace(),
                                    resource.getMetadata().getName(),
                                    resource.getStatus().getConsensusCapsules()),
                    ex);
            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withType(CONDITION_TYPE)
                                    .withStatus(ConditionStatus.False)
                                    .withLastTransitionTime(Instant.now().toString())
                                    .withMessage("Failed to update orbit '%s': %s"
                                            .formatted(resource.getSpec().getOrbit(), ex.getMessage()))
                                    .build())
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }

        log.infof("Updated orbit: %s", resource.getMetadata().getName());
        return edit(status)
                .with(s -> s.addToConditions(new ConditionBuilder()
                        .withType(CONDITION_TYPE)
                        .withStatus(ConditionStatus.True)
                        .withLastTransitionTime(Instant.now().toString())
                        .withMessage("Successfully updated orbit: %s"
                                .formatted(resource.getSpec().getOrbit()))
                        .build()))
                .build();
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
