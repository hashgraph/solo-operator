// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.generic.Nullable.orEmptyList;
import static com.hedera.solo.operator.library.generic.Nullable.orNull;

import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.SoftwareVersion;
import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.OpenTelemetryCollector;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * The `NowFrozenConsensusCapsules` class is responsible for updating the specifications of consensus capsules based on
 * the network operation and status.
 */
public class NowFrozenConsensusCapsules {
    final NetworkOperation networkOperation;
    final Map<Long, String> logging = new HashMap<>();
    final Map<Long, Map<String, String>> nodeProperties = new HashMap<>();
    final Map<Long, AddressBookEntry> addressBook = new HashMap<>();
    final String defaultLogging;
    final ImageRepositoryStatus consensus;
    final ImageRepositoryStatus backup;
    final ImageRepositoryStatus events;
    final ImageRepositoryStatus recordSidecar;
    final ImageRepositoryStatus record;
    final ImageRepositoryStatus otel;

    /**
     * Constructs a new `NowFrozenConsensusCapsules` instance with the given network operation and status.
     *
     * @param networkOperation The network operation containing the specifications to be updated.
     * @param status The current status of the network operation.
     */
    public NowFrozenConsensusCapsules(final NetworkOperation networkOperation, final NetworkOperationStatus status) {
        this(networkOperation.edit().withStatus(status).build());
    }

    /**
     * Constructs a new `NowFrozenConsensusCapsules` instance with the given network operation.
     *
     * @param networkOperation The network operation containing the specifications to be updated.
     */
    public NowFrozenConsensusCapsules(final NetworkOperation networkOperation) {
        this.networkOperation = networkOperation;
        orEmptyList(() -> networkOperation.getSpec().getNetworkUpgrade().getAddressBook())
                .forEach(entry -> addressBook.put(entry.getNodeId(), entry));
        ConsensusNodeChangeset changeset = orNull(() ->
                networkOperation.getSpec().getNetworkUpgrade().getChangesets().getConsensusNodeChangeset());
        if (changeset != null) {
            orEmptyList(changeset::getLogging)
                    .forEach(logs -> logging.putIfAbsent(logs.getNodeId(), logs.getConfiguration()));
            orEmptyList(changeset::getNodeProperties)
                    .forEach(props -> nodeProperties.putIfAbsent(props.getNodeId(), props.getConfiguration()));
        }
        defaultLogging = orNull(() -> networkOperation
                .getSpec()
                .getNetworkUpgrade()
                .getConfigurationFiles()
                .getLogging());
        SoftwareSourceStatus sourceStatus = networkOperation.getStatus().getSoftwareSourceStatus();
        consensus = selected(sourceStatus.getConsensusNode());
        backup = orNull(() -> selected(sourceStatus.getBackupUploader()));
        events = orNull(() -> selected(sourceStatus.getEventsStreamUploader()));
        recordSidecar = orNull(() -> selected(sourceStatus.getRecordStreamSidecarUploader()));
        record = orNull(() -> selected(sourceStatus.getRecordStreamUploader()));
        otel = orNull(() -> selected(sourceStatus.getOtelCollector()));
    }

    /**
     * Updates the given consensus capsule with the specified changes based on the network operation and status.
     *
     * @param consensusCapsule The consensus capsule to be updated.
     * @return The updated consensus capsule
     */
    public ConsensusCapsule update(final ConsensusCapsule consensusCapsule) {
        AddressBookEntry addressBookEntry =
                addressBook.get(consensusCapsule.getSpec().getNodeId());
        if (addressBookEntry == null
                || !Objects.equals(
                        networkOperation.getSpec().getOrbit(),
                        consensusCapsule.getSpec().getOrbit())) {
            return consensusCapsule;
        }
        Containers containers = consensusCapsule.getSpec().getPodProperties().getContainers();
        return edit(consensusCapsule)
                .with(spec -> spec.editSpec()
                        .withLog4j2Xml(logging(consensusCapsule))
                        .withNodeProperties(nodeProperties(consensusCapsule))
                        .withAccountId(addressBookEntry.getNodeAccount())
                        .withWeight(addressBookEntry.getWeight()))
                .with(builder -> builder.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .editConsensusNode()
                        .withSoftwareVersion(softwareVersion(
                                consensus, containers.getConsensusNode().getSoftwareVersion())))
                .with(builder -> backup == null
                        ? builder
                        : builder.editSpec()
                                .editPodProperties()
                                .editContainers()
                                .withBackupUploader(streamUploader(backup, containers.getBackupUploader())))
                .with(builder -> events == null
                        ? builder
                        : builder.editSpec()
                                .editPodProperties()
                                .editContainers()
                                .withEventsStreamUploader(streamUploader(events, containers.getEventsStreamUploader())))
                .with(builder -> recordSidecar == null
                        ? builder
                        : builder.editSpec()
                                .editPodProperties()
                                .editContainers()
                                .withRecordStreamSidecarFilesUploader(streamUploader(
                                        recordSidecar, containers.getRecordStreamSidecarFilesUploader())))
                .with(builder -> record == null
                        ? builder
                        : builder.editSpec()
                                .editPodProperties()
                                .editContainers()
                                .withRecordStreamUploader(streamUploader(record, containers.getRecordStreamUploader())))
                .with(builder -> otel == null
                        ? builder
                        : builder.editSpec()
                                .editPodProperties()
                                .editContainers()
                                .withOpenTelemetryCollector(
                                        otelCollector(otel, containers.getOpenTelemetryCollector())))
                .build();
    }

    /**
     * Gets the selected image repository status from a list of image repository statuses.
     *
     * @param list The list of image repository statuses to select from.
     * @return The selected image repository status
     */
    ImageRepositoryStatus selected(final List<ImageRepositoryStatus> list) {
        return list.stream()
                .filter(ImageRepositoryStatus::getSelectedForDeployment)
                .findAny()
                .orElseThrow();
    }

    /**
     * Updates the OpenTelemetry collector specifications based on the given status and existing configuration.
     *
     * @param status The image repository status containing the software version information.
     * @param otelCollector The existing OpenTelemetry collector configuration to be updated.
     * @return The updated OpenTelemetry collector configuration
     */
    OpenTelemetryCollector otelCollector(
            final ImageRepositoryStatus status, final OpenTelemetryCollector otelCollector) {
        if (status == null || otelCollector == null) {
            return null;
        }
        return edit(otelCollector)
                .with(builder ->
                        builder.withSoftwareVersion(softwareVersion(status, otelCollector.getSoftwareVersion())))
                .build();
    }

    /**
     * Updates the stream uploader specifications based on the given status and existing configuration.
     *
     * @param status The image repository status containing the software version information.
     * @param uploader The existing stream uploader configuration to be updated.
     * @return The updated stream uploader configuration
     */
    StreamUploader streamUploader(final ImageRepositoryStatus status, final StreamUploader uploader) {
        if (status == null || uploader == null) {
            return null;
        }
        return edit(uploader)
                .with(builder -> builder.withSoftwareVersion(softwareVersion(status, uploader.getSoftwareVersion())))
                .build();
    }

    /**
     * Updates the software version specifications based on the given status and existing configuration.
     *
     * @param status The image repository status containing the software version information.
     * @param softwareVersion The existing software version configuration to be updated.
     * @return The updated software version configuration
     */
    SoftwareVersion softwareVersion(final ImageRepositoryStatus status, final SoftwareVersion softwareVersion) {
        return edit(softwareVersion)
                .with(builder -> builder.withRepository(status.getRepository())
                        .withImageName(status.getImageName())
                        .withImageTag(status.getImageTag())
                        .withManifestHash(status.getManifestHash())
                        .withImagePullPolicy(SoftwareVersion.ImagePullPolicy.valueOf(
                                status.getImagePullPolicy().getValue()))
                        .withImagePullSecrets(status.getImagePullSecrets())
                        .withImageVerificationSpec(status.getImageVerificationSpec()))
                .build();
    }

    /**
     * Retrieves the logging configuration for the specified consensus capsule or uses the default logging if not set.
     *
     * @param consensusCapsule The consensus capsule to get the logging configuration for.
     * @return The logging configuration as a string
     */
    String logging(final ConsensusCapsule consensusCapsule) {
        return logging.getOrDefault(
                consensusCapsule.getSpec().getNodeId(),
                defaultLogging != null
                        ? defaultLogging
                        : consensusCapsule.getSpec().getLog4j2_xml());
    }

    /**
     * Retrieves the node properties configuration for the specified consensus capsule or uses the default properties if
     * not set.
     *
     * @param consensusCapsule The consensus capsule to get the node properties for.
     * @return A map of node properties
     */
    Map<String, String> nodeProperties(final ConsensusCapsule consensusCapsule) {
        return nodeProperties.getOrDefault(
                consensusCapsule.getSpec().getNodeId(),
                consensusCapsule.getSpec().getNodeProperties());
    }
}
