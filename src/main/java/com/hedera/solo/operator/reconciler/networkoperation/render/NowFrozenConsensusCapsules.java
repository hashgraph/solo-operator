// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.generic.Nullable.orEmptyList;
import static com.hedera.solo.operator.library.generic.Nullable.orNull;

import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.OpenTelemetryCollector;
import com.hedera.solo.operator.model.commonspec.SoftwareVersion;
import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

public class NowFrozenConsensusCapsules {
    final NetworkOperation networkOperation;
    final Map<Long, String> logging = new HashMap<>();
    final Map<Long, Map<String, String>> nodeProperties = new HashMap<>();
    final String defaultLogging;
    final ImageRepositoryStatus consensus;
    final ImageRepositoryStatus backup;
    final ImageRepositoryStatus events;
    final ImageRepositoryStatus recordSidecar;
    final ImageRepositoryStatus record;
    final ImageRepositoryStatus otel;

    public NowFrozenConsensusCapsules(final NetworkOperation networkOperation, final NetworkOperationStatus status) {
        this(networkOperation.edit().withStatus(status).build());
    }

    public NowFrozenConsensusCapsules(final NetworkOperation networkOperation) {
        this.networkOperation = networkOperation;
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

    public ConsensusCapsule update(final ConsensusCapsule consensusCapsule) {
        if (!Objects.equals(
                networkOperation.getSpec().getOrbit(),
                consensusCapsule.getSpec().getOrbit())) {
            return consensusCapsule;
        }
        Containers containers = consensusCapsule.getSpec().getPodProperties().getContainers();
        return edit(consensusCapsule)
                .with(spec -> spec.editSpec()
                        .withLog4j2Xml(logging(consensusCapsule))
                        .withNodeProperties(nodeProperties(consensusCapsule)))
                .with(builder -> builder.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .editConsensusNode()
                        .withSoftwareVersion(softwareVersion(
                                consensus, containers.getConsensusNode().getSoftwareVersion())))
                .with(builder -> builder.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .withBackupUploader(streamUploader(backup, containers.getBackupUploader())))
                .with(builder -> builder.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .withEventsStreamUploader(streamUploader(events, containers.getEventsStreamUploader())))
                .with(builder -> builder.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .withRecordStreamSidecarFilesUploader(
                                streamUploader(recordSidecar, containers.getRecordStreamSidecarFilesUploader())))
                .with(builder -> builder.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .withRecordStreamUploader(streamUploader(record, containers.getRecordStreamUploader())))
                .with(builder -> builder.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .withOpenTelemetryCollector(otelCollector(otel, containers.getOpenTelemetryCollector())))
                .build();
    }

    ImageRepositoryStatus selected(final List<ImageRepositoryStatus> list) {
        return list.stream()
                .filter(ImageRepositoryStatus::getSelectedForDeployment)
                .findAny()
                .orElseThrow();
    }

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

    StreamUploader streamUploader(final ImageRepositoryStatus status, final StreamUploader uploader) {
        if (status == null || uploader == null) {
            return null;
        }
        return edit(uploader)
                .with(builder -> builder.withSoftwareVersion(softwareVersion(status, uploader.getSoftwareVersion())))
                .build();
    }

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

    String logging(final ConsensusCapsule consensusCapsule) {
        return logging.getOrDefault(
                consensusCapsule.getSpec().getNodeId(),
                defaultLogging != null
                        ? defaultLogging
                        : consensusCapsule.getSpec().getLog4j2_xml());
    }

    Map<String, String> nodeProperties(final ConsensusCapsule consensusCapsule) {
        return nodeProperties.getOrDefault(
                consensusCapsule.getSpec().getNodeId(),
                consensusCapsule.getSpec().getNodeProperties());
    }
}
