// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.ConfigurationFiles;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyOrbitConfigurationChangeStep;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * CheckOrbitConfigurationStep checks that orbit configuration are rolled out
 *
 * <p>It needs to inspect the CR and performs the necessary checks to ensure that the orbit configuration is deployed.
 */
public class CheckOrbitConfigurationStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE = new StepConditionType("ConfigurationVerified", "Orbit");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully verified orbit configuration";
    public static final String FAIL_STATUS_MESSAGE = "Failed to verify orbit configuration: %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    public CheckOrbitConfigurationStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return !NetworkOperationStepUtil.isNetworkUpgrade(resource)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        return !NetworkOperationStepUtil.isNetworkUpgrade(resource)
                || !ApplyOrbitConfigurationChangeStep.isDone(status)
                || status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        try {
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(status, "NetworkOperationStatus status is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> context is required");
            Objects.requireNonNull(resource.getSpec(), "NetworkOperation spec is required");
            Objects.requireNonNull(
                    resource.getSpec().getNetworkUpgrade(), "NetworkOperation network-upgrade spec is required");

            final Orbit orbit =
                    customResourceManager.orbitByName(resource.getSpec().getOrbit());
            if (orbit == null) {
                throw new ProcessorException(
                        "Orbit '%s' not found".formatted(resource.getSpec().getOrbit()));
            }

            // check address book configuration
            if (resource.getSpec().getNetworkUpgrade().getAddressBook() != null) {
                if (!orbit.getSpec()
                        .getConsensus()
                        .getGenesis()
                        .getAddressBook()
                        .getExternalAddresses()
                        .equals(resource.getSpec().getNetworkUpgrade().getAddressBook())) {
                    throw new ProcessorException("Address book entries have not been updated");
                }
            }

            ConfigurationFiles configurationFiles =
                    resource.getSpec().getNetworkUpgrade().getConfigurationFiles();
            if (configurationFiles != null) {
                // check settings.txt
                if (configurationFiles.getNetworkSettings() != null) {
                    if (!resource.getSpec()
                            .getNetworkUpgrade()
                            .getConfigurationFiles()
                            .getNetworkSettings()
                            .equals(orbit.getSpec().getConsensus().getNetworkSettings())) {
                        throw new ProcessorException("Network settings (settings.txt) has not been updated");
                    }
                }

                // check application.properties
                if (configurationFiles.getApplicationProperties() != null) {
                    if (!resource.getSpec()
                            .getNetworkUpgrade()
                            .getConfigurationFiles()
                            .getApplicationProperties()
                            .equals(orbit.getSpec().getConsensus().getApplicationConfiguration())) {
                        throw new ProcessorException(
                                "Application properties (application.properties) has not been updated");
                    }
                }

                // check api-permission.properties
                if (configurationFiles.getPermissionConfiguration() != null) {
                    if (!resource.getSpec()
                            .getNetworkUpgrade()
                            .getConfigurationFiles()
                            .getPermissionConfiguration()
                            .equals(orbit.getSpec().getConsensus().getPermissionConfiguration())) {
                        throw new ProcessorException(
                                "API permission properties (api-permission.properties) has not been updated");
                    }
                }
            }

            // TODO check logging configuration #592

            log.infof(
                    "Orbit configuration check was successful for '%s/%s' [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getMode());

            return NetworkOperationStepUtil.prepareStatus(
                    status,
                    List.of(new ConditionBuilder()
                            .withType(CONDITION_TYPE.type())
                            .withReason(CONDITION_TYPE.reason())
                            .withLastTransitionTime(timestamp())
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withMessage(SUCCESS_STATUS_MESSAGE)
                            .withStatus(ConditionStatus.True)
                            .build()));
        } catch (Throwable t) {
            final String failedMessage = FAIL_STATUS_MESSAGE.formatted(t.getMessage());
            log.error(failedMessage, t);
            return NetworkOperationStepUtil.prepareFailed(resource, status, failedMessage, CONDITION_TYPE);
        }
    }
}
