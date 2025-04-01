// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.NodeLoggingConfiguration;
import com.hedera.solo.operator.model.commonspec.NodeProperties;
import com.hedera.solo.operator.model.commonspec.SoftwareVersion;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.utils.ConsensusPodSidecarUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import org.jboss.logging.Logger;

/**
 * CheckConsensusPodConfigurationStep checks that consensus capsule configuration are rolled out during network upgrade.
 *
 * <p>It needs to inspect the CR and performs the necessary checks to ensure that the consensus nodes are running with
 * correct version and configuration files.
 */
public class CheckConsensusCapsuleConfigurationStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE =
            new StepConditionType("ConfigurationVerified", "ConsensusCapsule");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully verified consensus capsule configuration";
    public static final String FAIL_STATUS_MESSAGE = "Failed to verify consensus capsule configuration: %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    public CheckConsensusCapsuleConfigurationStep(CustomResourceManager customResourceManager) {
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

            // check consensus changeset configuration
            final List<ConsensusCapsule> consensusCapsules = customResourceManager.consensusCapsules(
                    resource.getMetadata().getNamespace());
            if (consensusCapsules.isEmpty()) {
                throw new ProcessorException("Consensus capsule not found for network operation: %s/%s"
                        .formatted(
                                resource.getMetadata().getNamespace(),
                                resource.getMetadata().getName()));
            }

            for (ConsensusCapsule consensusCapsule : consensusCapsules) {
                final String nodeName = Metadata.prepareNodeName(consensusCapsule);

                // check consensus capsule configuration
                Optional<ImageRepositoryStatus> selectedVersion =
                        resource.getStatus().getSoftwareSourceStatus().getConsensusNode().stream()
                                .filter(ImageRepositoryStatus::getSelectedForDeployment)
                                .findFirst();

                if (selectedVersion.isEmpty()) {
                    throw new ProcessorException(
                            "Consensus capsule software version not selected for consensus node: %s"
                                    .formatted(nodeName));
                }

                if (!matchImageVersion(
                        selectedVersion.get(),
                        consensusCapsule
                                .getSpec()
                                .getPodProperties()
                                .getContainers()
                                .getConsensusNode()
                                .getSoftwareVersion())) {
                    throw new ProcessorException(
                            "Consensus capsule software version mismatch for consensus node: %s".formatted(nodeName));
                }

                log.infof(
                        "Consensus capsule software version matched for consensus node: %s [ version: %s]",
                        nodeName, toImageVersion(selectedVersion.get()));

                // check node properties changeset configuration
                if (resource.getSpec()
                                .getNetworkUpgrade()
                                .getChangesets()
                                .getConsensusNodeChangeset()
                                .getNodeProperties()
                        != null) {

                    for (NodeProperties prop : resource.getSpec()
                            .getNetworkUpgrade()
                            .getChangesets()
                            .getConsensusNodeChangeset()
                            .getNodeProperties()) {

                        if (Objects.equals(
                                prop.getNodeId(), consensusCapsule.getSpec().getNodeId())) {
                            final Map<String, String> nodeProperties = prop.getConfiguration();
                            if (!nodeProperties.equals(
                                    consensusCapsule.getSpec().getNodeProperties())) {
                                throw new ProcessorException(
                                        "Node properties configuration mismatch for consensus node: %s"
                                                .formatted(nodeName));
                            }

                            log.infof(
                                    "Node properties configuration matched for consensus node: %s [ properties: %s]",
                                    nodeName, nodeProperties);

                            break;
                        }
                    }
                }

                // check logging configuration
                if (resource.getSpec()
                                .getNetworkUpgrade()
                                .getChangesets()
                                .getConsensusNodeChangeset()
                                .getLogging()
                        != null) {

                    for (NodeLoggingConfiguration conf : resource.getSpec()
                            .getNetworkUpgrade()
                            .getChangesets()
                            .getConsensusNodeChangeset()
                            .getLogging()) {

                        if (Objects.equals(
                                conf.getNodeId(), consensusCapsule.getSpec().getNodeId())) {
                            final String loggingProperties = conf.getConfiguration();
                            if (!loggingProperties.equals(
                                    consensusCapsule.getSpec().getLog4j2_xml())) {
                                throw new ProcessorException(
                                        "Logging configuration mismatch for consensus node: %s".formatted(nodeName));
                            }

                            log.infof("Logging configuration matched for consensus node: %s", nodeName);

                            break;
                        }
                    }
                }

                // check record-stream-uploader changeset configuration
                if (ConsensusPodSidecarUtil.hasStreamUploaderChangeset(
                        resource, Constants.TPL_VAR_RECORD_STREAM_UPLOADER)) {
                    selectedVersion = resource.getStatus().getSoftwareSourceStatus().getRecordStreamUploader().stream()
                            .filter(ImageRepositoryStatus::getSelectedForDeployment)
                            .findFirst();

                    if (selectedVersion.isEmpty()) {
                        throw new ProcessorException(
                                "Record stream uploader software version not selected for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    if (!matchImageVersion(
                            selectedVersion.get(),
                            consensusCapsule
                                    .getSpec()
                                    .getPodProperties()
                                    .getContainers()
                                    .getRecordStreamUploader()
                                    .getSoftwareVersion())) {
                        throw new ProcessorException(
                                "Record stream uploader software version mismatch for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    log.infof(
                            "Record stream uploader software version matched for consensus node: %s [ version: %s]",
                            nodeName, toImageVersion(selectedVersion.get()));
                }

                // check event-stream-uploader changeset configuration
                if (ConsensusPodSidecarUtil.hasStreamUploaderChangeset(
                        resource, Constants.TPL_VAR_EVENT_STREAM_UPLOADER)) {
                    selectedVersion = resource.getStatus().getSoftwareSourceStatus().getEventsStreamUploader().stream()
                            .filter(ImageRepositoryStatus::getSelectedForDeployment)
                            .findFirst();

                    if (selectedVersion.isEmpty()) {
                        throw new ProcessorException(
                                "Events stream uploader software version not selected for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    if (!matchImageVersion(
                            selectedVersion.get(),
                            consensusCapsule
                                    .getSpec()
                                    .getPodProperties()
                                    .getContainers()
                                    .getEventsStreamUploader()
                                    .getSoftwareVersion())) {
                        throw new ProcessorException(
                                "Events stream uploader software version mismatch for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    log.infof(
                            "Events stream uploader software version matched for consensus node: %s [ version: %s]",
                            nodeName, toImageVersion(selectedVersion.get()));
                }

                // check backup-uploader changeset configuration
                if (ConsensusPodSidecarUtil.hasStreamUploaderChangeset(resource, Constants.TPL_VAR_BACKUP_UPLOADER)) {
                    selectedVersion = resource.getStatus().getSoftwareSourceStatus().getBackupUploader().stream()
                            .filter(ImageRepositoryStatus::getSelectedForDeployment)
                            .findFirst();

                    if (selectedVersion.isEmpty()) {
                        throw new ProcessorException(
                                "Backup uploader software version not selected for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    if (!matchImageVersion(
                            selectedVersion.get(),
                            consensusCapsule
                                    .getSpec()
                                    .getPodProperties()
                                    .getContainers()
                                    .getBackupUploader()
                                    .getSoftwareVersion())) {
                        throw new ProcessorException(
                                "Backup uploader software version mismatch for consensus node: %s".formatted(nodeName));
                    }

                    log.infof(
                            "Backup uploader software version matched for consensus node: %s [ version: %s]",
                            nodeName, toImageVersion(selectedVersion.get()));
                }

                // check record-stream-sidecar changeset configuration
                if (ConsensusPodSidecarUtil.hasStreamUploaderChangeset(
                        resource, Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)) {
                    selectedVersion =
                            resource.getStatus().getSoftwareSourceStatus().getRecordStreamSidecarUploader().stream()
                                    .filter(ImageRepositoryStatus::getSelectedForDeployment)
                                    .findFirst();

                    if (selectedVersion.isEmpty()) {
                        throw new ProcessorException(
                                "Record stream sidecar uploader software version not selected for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    if (!matchImageVersion(
                            selectedVersion.get(),
                            consensusCapsule
                                    .getSpec()
                                    .getPodProperties()
                                    .getContainers()
                                    .getRecordStreamSidecarFilesUploader()
                                    .getSoftwareVersion())) {
                        throw new ProcessorException(
                                "Record stream sidecar uploader software version mismatch for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    log.infof(
                            "Record stream sidecar uploader software version matched for consensus node: %s [ version: %s]",
                            nodeName, toImageVersion(selectedVersion.get()));
                }

                // check otel-collector changeset configuration
                if (ConsensusPodSidecarUtil.hasOtelChangeset(resource)) {
                    selectedVersion = resource.getStatus().getSoftwareSourceStatus().getOtelCollector().stream()
                            .filter(ImageRepositoryStatus::getSelectedForDeployment)
                            .findFirst();

                    if (selectedVersion.isEmpty()) {
                        throw new ProcessorException(
                                "Otel collector sidecar software version not selected for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    if (!matchImageVersion(
                            selectedVersion.get(),
                            consensusCapsule
                                    .getSpec()
                                    .getPodProperties()
                                    .getContainers()
                                    .getOpenTelemetryCollector()
                                    .getSoftwareVersion())) {
                        throw new ProcessorException(
                                "Open telemetry collector software version mismatch for consensus node: %s"
                                        .formatted(nodeName));
                    }

                    log.infof(
                            "Open telemetry sidecar software version matched for consensus node: %s [ version: %s]",
                            nodeName, toImageVersion(selectedVersion.get()));
                }
            }

            log.infof(
                    "Consensus capsule configuration check was successful for '%s/%s' [ %s ]",
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
            final String failedMessage = String.format(FAIL_STATUS_MESSAGE, t.getMessage());
            log.errorf(t, failedMessage);
            return NetworkOperationStepUtil.prepareFailed(resource, status, failedMessage, CONDITION_TYPE);
        }
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    static boolean matchImageVersion(ImageRepositoryStatus src, SoftwareVersion dest) {
        return dest.getImageName().equals(src.getImageName())
                && dest.getImageTag().equals(src.getImageTag())
                && dest.getRepository().equals(src.getRepository());
    }

    static String toImageVersion(ImageRepositoryStatus status) {
        return "%s/%s:%s".formatted(status.getRepository(), status.getImageName(), status.getImageTag());
    }
}
