// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.generic.Nullable.orEmptyList;
import static com.hedera.solo.operator.library.generic.Nullable.orNull;
import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.core.type.TypeReference;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.NodeLoggingConfiguration;
import com.hedera.solo.operator.model.commonspec.NodeProperties;
import com.hedera.solo.operator.model.commonspec.SoftwareVersion;
import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.ConsensusNode;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.Changesets;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.networkoperation.render.NowFrozenConsensusCapsules;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import com.hedera.solo.operator.test.fixtures.fuzz.KubernetesResourceFactory;
import java.lang.reflect.Type;
import java.util.HashSet;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.function.Consumer;
import java.util.function.Function;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import java.util.stream.LongStream;
import java.util.stream.Stream;
import org.junit.jupiter.api.RepeatedTest;

public class NowFrozenConsensusCapsulesUnitTest {
    BeanFactory factory = beanFactory()
            .supplier(new TypeReference<List<ImageRepositoryStatus>>() {}, this::imageRepositoryStatusList);

    BeanFactory beanFactory() {
        return new KubernetesResourceFactory();
    }

    @RepeatedTest(10)
    public void test() {
        Set<Long> nodeIds = LongStream.range(0, 15).boxed().collect(Collectors.toSet());
        NetworkOperation networkOperation = networkOperation(nodeIds);
        NetworkUpgrade networkUpgrade = networkOperation.getSpec().getNetworkUpgrade();
        ConsensusNodeChangeset changeset = networkUpgrade.getChangesets().getConsensusNodeChangeset();
        Map<Long, String> logging = orEmptyList(changeset::getLogging).stream()
                .collect(Collectors.toMap(
                        NodeLoggingConfiguration::getNodeId, NodeLoggingConfiguration::getConfiguration));
        Map<Long, Map<String, String>> nodeProperties = orEmptyList(changeset::getNodeProperties).stream()
                .collect(Collectors.toMap(NodeProperties::getNodeId, NodeProperties::getConfiguration));
        String defaultLogging =
                orNull(() -> networkUpgrade.getConfigurationFiles().getLogging());
        Stream<ConsensusCapsule> orig = nodeIds.stream().map(id -> consensusCapsule(networkOperation, id));
        Stream<Update> update = update(networkOperation, orig);
        Set<Long> nodeIdsToUpdate = orEmptyList(networkUpgrade::getAddressBook).stream()
                .map(AddressBookEntry::getNodeId)
                .collect(Collectors.toSet());
        for (Iterator<Update> it = update.iterator(); it.hasNext(); ) {
            Update pair = it.next();
            ConsensusCapsule initial = pair.initial;
            ConsensusCapsule updated = pair.updated;

            long nodeId = initial.getSpec().getNodeId();
            assertThat(updated.getSpec().getNodeId()).isEqualTo(nodeId);

            if (nodeIdsToUpdate.contains(nodeId)) {
                assertThat(updated.getSpec().getLog4j2_xml())
                        .isEqualTo(logging.getOrDefault(
                                nodeId,
                                defaultLogging != null
                                        ? defaultLogging
                                        : initial.getSpec().getLog4j2_xml()));
                assertThat(updated.getSpec().getNodeProperties())
                        .isEqualTo(nodeProperties.getOrDefault(
                                nodeId, initial.getSpec().getNodeProperties()));
                match(
                        networkOperation.getStatus().getSoftwareSourceStatus().getConsensusNode(),
                        updated.getSpec()
                                .getPodProperties()
                                .getContainers()
                                .getConsensusNode()
                                .getSoftwareVersion());
                maybeMatch(
                        networkOperation,
                        initial,
                        updated,
                        SoftwareSourceStatus::getBackupUploader,
                        Containers::getBackupUploader);
                maybeMatch(
                        networkOperation,
                        initial,
                        updated,
                        SoftwareSourceStatus::getEventsStreamUploader,
                        Containers::getEventsStreamUploader);
                maybeMatch(
                        networkOperation,
                        initial,
                        updated,
                        SoftwareSourceStatus::getRecordStreamSidecarUploader,
                        Containers::getRecordStreamSidecarFilesUploader);
                maybeMatch(
                        networkOperation,
                        initial,
                        updated,
                        SoftwareSourceStatus::getRecordStreamUploader,
                        Containers::getRecordStreamUploader);
            } else {
                assertThat(updated.getSpec()).isEqualTo(initial.getSpec());
            }
        }
    }

    record Update(ConsensusCapsule initial, ConsensusCapsule updated) {}

    Stream<Update> update(final NetworkOperation networkOperation, final Stream<ConsensusCapsule> capsules) {
        Set<Long> exclude =
                new HashSet<>(orEmptyList(() -> networkOperation.getStatus().getUpgradedNodes()));
        Set<Long> include = orEmptyList(
                        () -> networkOperation.getSpec().getNetworkUpgrade().getAddressBook())
                .stream()
                .map(AddressBookEntry::getNodeId)
                .collect(Collectors.toSet());
        NowFrozenConsensusCapsules nowFrozen = new NowFrozenConsensusCapsules(networkOperation);
        return capsules.map(initial -> new Update(
                initial,
                include.contains(initial.getSpec().getNodeId())
                                && !exclude.contains(initial.getSpec().getNodeId())
                        ? nowFrozen.update(initial)
                        : initial));
    }

    void maybeMatch(
            final NetworkOperation networkOperation,
            final ConsensusCapsule initial,
            final ConsensusCapsule updated,
            final Function<SoftwareSourceStatus, List<ImageRepositoryStatus>> getStatus,
            final Function<Containers, StreamUploader> getContainer) {
        maybeMatch(
                getStatus.apply(networkOperation.getStatus().getSoftwareSourceStatus()),
                getContainer.apply(initial.getSpec().getPodProperties().getContainers()),
                orNull(() -> getContainer
                        .apply(updated.getSpec().getPodProperties().getContainers())
                        .getSoftwareVersion()));
    }

    void maybeMatch(
            final List<ImageRepositoryStatus> list, final StreamUploader initial, final SoftwareVersion actual) {
        if (list != null && initial != null) {
            match(list, actual);
        }
    }

    void match(final List<ImageRepositoryStatus> list, final SoftwareVersion actual) {
        ImageRepositoryStatus expected = list.stream()
                .filter(ImageRepositoryStatus::getSelectedForDeployment)
                .findFirst()
                .orElse(null);
        assertThat(expected).isNotNull();
        assertThat(expected.getValidity()).isEqualTo(ImageRepositoryStatus.Validity.Valid);
        assertThat(expected.getSelectedForDeployment()).isTrue();
        assertThat(actual).isNotNull();
        assertThat(expected.getRepository()).isEqualTo(actual.getRepository());
        assertThat(expected.getImageName()).isEqualTo(actual.getImageName());
        assertThat(expected.getImageTag()).isEqualTo(actual.getImageTag());
        assertThat(expected.getManifestHash()).isEqualTo(actual.getManifestHash());
        assertThat(expected.getImagePullSecrets()).isEqualTo(actual.getImagePullSecrets());
        assertThat(expected.getImagePullPolicy().getValue())
                .isEqualTo(actual.getImagePullPolicy().getValue());
        assertThat(expected.getImageVerificationSpec()).isEqualTo(actual.getImageVerificationSpec());
    }

    ConsensusCapsule consensusCapsule(final NetworkOperation networkOperation, final long nodeId) {
        ConsensusCapsule consensusCapsule = factory.nonNull(ConsensusCapsule.class);
        if (consensusCapsule.getSpec() == null) {
            consensusCapsule.setSpec(factory.nonNull(ConsensusCapsuleSpec.class));
        }
        consensusCapsule.getSpec().setOrbit(networkOperation.getSpec().getOrbit());
        consensusCapsule.getSpec().setNodeId(nodeId);
        if (consensusCapsule.getSpec().getPodProperties() == null) {
            consensusCapsule.getSpec().setPodProperties(factory.nonNull(PodProperties.class));
        }
        if (consensusCapsule.getSpec().getPodProperties().getContainers() == null) {
            consensusCapsule.getSpec().getPodProperties().setContainers(factory.nonNull(Containers.class));
        }
        if (consensusCapsule.getSpec().getPodProperties().getContainers().getConsensusNode() == null) {
            consensusCapsule
                    .getSpec()
                    .getPodProperties()
                    .getContainers()
                    .setConsensusNode(factory.nonNull(ConsensusNode.class));
        }
        Containers containers = consensusCapsule.getSpec().getPodProperties().getContainers();
        softwareVersion(
                containers.getConsensusNode()::getSoftwareVersion, containers.getConsensusNode()::setSoftwareVersion);
        streamUploader(containers.getBackupUploader());
        streamUploader(containers.getEventsStreamUploader());
        streamUploader(containers.getRecordStreamSidecarFilesUploader());
        streamUploader(containers.getRecordStreamUploader());
        return consensusCapsule;
    }

    void streamUploader(final StreamUploader streamUploader) {
        if (streamUploader != null) {
            softwareVersion(streamUploader::getSoftwareVersion, streamUploader::setSoftwareVersion);
        }
    }

    void softwareVersion(final Supplier<SoftwareVersion> getter, final Consumer<SoftwareVersion> setter) {
        if (getter.get() == null) {
            setter.accept(factory.nonNull(SoftwareVersion.class));
        }
    }

    NetworkOperation networkOperation(final Set<Long> nodeIds) {
        NetworkOperation networkOperation = factory.nonNull(NetworkOperation.class);
        if (networkOperation.getSpec() == null) {
            networkOperation.setSpec(factory.nonNull(NetworkOperationSpec.class));
        }
        if (networkOperation.getSpec().getNetworkUpgrade() == null) {
            networkOperation.getSpec().setNetworkUpgrade(factory.nonNull(NetworkUpgrade.class));
        }
        NetworkUpgrade networkUpgrade = networkOperation.getSpec().getNetworkUpgrade();
        if (networkUpgrade.getChangesets() == null) {
            networkUpgrade.setChangesets(factory.nonNull(Changesets.class));
        }
        if (networkUpgrade.getAddressBook() != null) {
            networkUpgrade.setAddressBook(networkUpgrade.getAddressBook().stream()
                    .filter(Objects::nonNull)
                    .toList());
            Iterator<Long> id = nodeIds.iterator();
            Iterator<AddressBookEntry> addr = networkUpgrade.getAddressBook().iterator();
            while (id.hasNext() && addr.hasNext()) {
                addr.next().setNodeId(id.next());
            }
        }
        if (networkUpgrade.getChangesets().getConsensusNodeChangeset() == null) {
            networkUpgrade.getChangesets().setConsensusNodeChangeset(factory.nonNull(ConsensusNodeChangeset.class));
        }
        ConsensusNodeChangeset changeset = networkUpgrade.getChangesets().getConsensusNodeChangeset();
        if (changeset.getLogging() != null) {
            changeset.setLogging(nodeIds.stream()
                    .limit(5)
                    .map(this::nodeLoggingConfigurations)
                    .toList());
        }
        if (changeset.getNodeProperties() != null) {
            changeset.setNodeProperties(
                    nodeIds.stream().limit(5).map(this::nodeProperties).toList());
        }
        if (networkOperation.getStatus() == null) {
            networkOperation.setStatus(factory.nonNull(NetworkOperationStatus.class));
        }
        if (networkOperation.getStatus().getSoftwareSourceStatus() == null) {
            networkOperation.getStatus().setSoftwareSourceStatus(factory.nonNull(SoftwareSourceStatus.class));
        }
        if (networkOperation.getStatus().getSoftwareSourceStatus().getConsensusNode() == null) {
            networkOperation
                    .getStatus()
                    .getSoftwareSourceStatus()
                    .setConsensusNode(factory.nonNull(new TypeReference<List<ImageRepositoryStatus>>() {}));
        }
        return networkOperation;
    }

    List<ImageRepositoryStatus> imageRepositoryStatusList(final BeanFactory factory, final Type type) {
        List<ImageRepositoryStatus> list = factory.nonNulls(ImageRepositoryStatus.class)
                .limit(factory.random().nextInt(1, 8))
                .toList();
        list.forEach(status -> status.setSelectedForDeployment(false));
        ImageRepositoryStatus selected = list.get(factory.random().nextInt(list.size()));
        selected.setValidity(ImageRepositoryStatus.Validity.Valid);
        selected.setSelectedForDeployment(true);
        if (selected.getImagePullPolicy() == null) {
            selected.setImagePullPolicy(factory.nonNull(ImageRepositoryStatus.ImagePullPolicy.class));
        }
        return list;
    }

    NodeLoggingConfiguration nodeLoggingConfigurations(final long nodeId) {
        return edit(new NodeLoggingConfiguration())
                .with(builder -> builder.withNodeId(nodeId).withConfiguration(factory.nonNull(String.class)))
                .build();
    }

    NodeProperties nodeProperties(final long nodeId) {
        return edit(new NodeProperties())
                .with(builder -> builder.withNodeId(nodeId)
                        .withConfiguration(factory.nonNull(new TypeReference<Map<String, String>>() {})))
                .build();
    }
}
