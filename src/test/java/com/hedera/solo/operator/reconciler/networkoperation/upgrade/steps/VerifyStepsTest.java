// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.github.tomakehurst.wiremock.client.WireMock.get;
import static com.github.tomakehurst.wiremock.client.WireMock.head;
import static com.github.tomakehurst.wiremock.client.WireMock.including;
import static com.github.tomakehurst.wiremock.client.WireMock.notFound;
import static com.github.tomakehurst.wiremock.client.WireMock.ok;
import static com.github.tomakehurst.wiremock.client.WireMock.urlMatching;
import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.NotFound;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.Unknown;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.Valid;
import static java.nio.charset.StandardCharsets.UTF_8;
import static org.assertj.core.api.Assertions.assertThat;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.hedera.solo.operator.library.data.Digest;
import com.hedera.solo.operator.library.processor.ProcessorTask;
import com.hedera.solo.operator.model.commonspec.ImageRepository;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.ImageVerificationSpec;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatusBuilder;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.Changesets;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.ExecuteImmediateTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.FreezeScheduledTask;
import com.hedera.solo.operator.test.fixtures.wiremock.WireMockServerFactory;
import com.hedera.solo.operator.test.fixtures.wiremock.WithWireMockCert;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.IndexedResourceCache;
import io.javaoperatorsdk.operator.api.reconciler.RetryInfo;
import io.javaoperatorsdk.operator.api.reconciler.dependent.managed.ManagedWorkflowAndDependentResourceContext;
import io.javaoperatorsdk.operator.processing.event.EventSourceRetriever;
import java.io.IOException;
import java.security.GeneralSecurityException;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.ExecutorService;
import java.util.stream.Collectors;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@WithWireMockCert
public class VerifyStepsTest {
    Context<NetworkOperation> context = new Ctx();
    WireMockServer wm;

    @BeforeEach
    public void setup(final WireMockServerFactory wireMockServerFactory) throws GeneralSecurityException, IOException {
        wm = wireMockServerFactory.wm();
        wm.stubFor(get(urlMatching("/v2/?")).willReturn(ok()));
        wm.stubFor(head(urlMatching("/v2(/.*)?/blobs/.*")).willReturn(ok()));
    }

    @Test
    void testNotFound() {
        NetworkOperation networkOperation = networkOperation();
        wm.stubFor(get(urlMatching("/v2/.*")).willReturn(notFound()));

        // verify
        allNotFound(networkOperation, new ExecuteImmediateTask(), 1);

        // re-verify
        allNotFound(networkOperation, new FreezeScheduledTask(), 2);

        // re-verify (with failed status)
        networkOperation.setStatus(new NetworkOperationStatusBuilder()
                .withPhase(NetworkOperationStatus.Phase.Failed)
                .build());
        allNotFound(networkOperation, new FreezeScheduledTask(), 0);
    }

    @Test
    void testUnknown() {
        NetworkOperation networkOperation = networkOperation();
        wm.stubFor(get(urlMatching("/v2/.*/manifests/.*"))
                .willReturn(ok().withHeader("Content-Type", "application/vnd.docker.distribution.manifest.v1+prettyjws")
                        .withBody("{}")));

        // verify
        allUnknown(networkOperation, new ExecuteImmediateTask(), 1);

        // re-verify
        allUnknown(networkOperation, new FreezeScheduledTask(), 2);

        // re-verify (with failed status)
        networkOperation.setStatus(new NetworkOperationStatusBuilder()
                .withPhase(NetworkOperationStatus.Phase.Failed)
                .build());
        allUnknown(networkOperation, new FreezeScheduledTask(), 0);
    }

    @Test
    void testValid() {
        NetworkOperation networkOperation = networkOperation();
        stubsFor(networkOperation);

        // verify
        allMatch(networkOperation, new ExecuteImmediateTask(), 6);

        // re-verify
        allMatch(networkOperation, new FreezeScheduledTask(), 12);

        // re-verify (with empty status)
        networkOperation.setStatus(new NetworkOperationStatusBuilder()
                .withPhase(NetworkOperationStatus.Phase.Failed)
                .build());
        allMatch(networkOperation, new FreezeScheduledTask(), 0);
    }

    void stubsFor(final NetworkOperation networkOperation) {
        Changesets changesets = networkOperation.getSpec().getNetworkUpgrade().getChangesets();
        stubsFor(changesets.getConsensusNodeChangeset().getSoftwareVersionSource());
        stubsFor(changesets.getBackupUploaderChangeset().getSoftwareVersionSource());
        stubsFor(changesets.getEventsStreamUploaderChangeset().getSoftwareVersionSource());
        stubsFor(changesets.getRecordStreamSidecarUploaderChangeset().getSoftwareVersionSource());
        stubsFor(changesets.getRecordStreamUploaderChangeset().getSoftwareVersionSource());
        stubsFor(changesets.getOtelCollector().getSoftwareVersionSource());
    }

    void stubsFor(final SoftwareVersionSource source) {
        for (ImageRepository repository : source.getImageRepositories()) {
            for (ImageVerificationSpec spec : source.getImageVerificationSpec()) {
                stubsFor(repository, spec);
            }
        }
    }

    void stubsFor(final ImageRepository repository, final ImageVerificationSpec spec) {
        String body = spec.getLayerHashes().stream()
                .map(digest ->
                        "{\"mediaType\":\"application/vnd.oci.image.layer.v1.tar+gzip\",\"digest\":\"" + digest + "\"}")
                .collect(Collectors.joining(",", "{\"layers\":[", "]}"));
        String digest = "sha256:" + Digest.SHA_256.hexOf(body.getBytes(UTF_8));
        wm.stubFor(get("/v2/" + repository.getImageName() + "/manifests/" + digest)
                .withHeader("Accept", including("application/vnd.oci.image.manifest.v1+json"))
                .willReturn(ok().withHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json")
                        .withBody(body)));
        wm.stubFor(get("/v2/" + repository.getImageName() + "/manifests/" + repository.getImageTag())
                .withHeader("Accept", including("application/vnd.oci.image.index.v1+json"))
                .willReturn(ok().withHeader("Content-Type", "application/vnd.oci.image.index.v1+json")
                        .withBody(
                                """
                                        {
                                          "manifests": [
                                            {
                                              "mediaType": "application/vnd.oci.image.manifest.v1+json",
                                              "digest": "%s",
                                              "platform": {
                                                "os": "%s",
                                                "architecture": "%s"
                                              }
                                            }
                                          ]
                                        }
                                        """
                                        .formatted(digest, spec.getOs(), spec.getArchitecture()))));
    }

    NetworkOperation networkOperation() {
        return edit(new NetworkOperation())
                .with(consensus -> consensus
                        .editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editConsensusNodeChangeset()
                        .withSoftwareVersionSource(
                                softwareVersionSource("consensus", "1.0", "darwin", "amd64", "hash:0", "hash:1")))
                .with(backupUploader -> backupUploader
                        .editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editBackupUploaderChangeset()
                        .withSoftwareVersionSource(softwareVersionSource("backup", "2.0", "darwin", "arm64", "hash:2")))
                .with(eventStreamUploader -> eventStreamUploader
                        .editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editEventsStreamUploaderChangeset()
                        .withSoftwareVersionSource(softwareVersionSource("event", "3.0", "linux", "amd64", "hash:3")))
                .with(recordStreamSidecarUploader -> recordStreamSidecarUploader
                        .editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editRecordStreamSidecarUploaderChangeset()
                        .withSoftwareVersionSource(
                                softwareVersionSource("recordSidecar", "4.0", "linux", "arm64", "hash:4")))
                .with(recordStreamUploader -> recordStreamUploader
                        .editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editRecordStreamUploaderChangeset()
                        .withSoftwareVersionSource(
                                softwareVersionSource("record", "5.0", "windows", "amd64", "hash:5", "hash:6")))
                .with(otel -> otel.editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editOtelCollector()
                        .withSoftwareVersionSource(softwareVersionSource("otel", "6.0", "windows", "amd64", "hash:7")))
                .with(status -> status.editStatus().withPhase(NetworkOperationStatus.Phase.Pending))
                .build();
    }

    void allNotFound(
            final NetworkOperation networkOperation,
            final ProcessorTask<NetworkOperation, NetworkOperationStatus> task,
            final int totalConditions) {
        allMatch(
                networkOperation,
                task.execute(networkOperation, context).getStatus(),
                ConditionStatus.False,
                NotFound,
                totalConditions);
    }

    void allUnknown(
            final NetworkOperation networkOperation,
            final ProcessorTask<NetworkOperation, NetworkOperationStatus> task,
            final int totalConditions) {
        allMatch(
                networkOperation,
                task.execute(networkOperation, context).getStatus(),
                ConditionStatus.False,
                Unknown,
                totalConditions);
    }

    void allMatch(
            final NetworkOperation networkOperation,
            final ProcessorTask<NetworkOperation, NetworkOperationStatus> task,
            final int totalConditions) {
        allMatch(
                networkOperation,
                task.execute(networkOperation, context).getStatus(),
                ConditionStatus.True,
                Valid,
                totalConditions);
    }

    void allMatch(
            final NetworkOperation networkOperation,
            final NetworkOperationStatus status,
            final String conditionStatus,
            final ImageRepositoryStatus.Validity validity,
            final int totalConditions) {
        if (totalConditions == 0) {
            assertThat(status.getConditions()).isNull();
            return;
        }

        assertThat(status.getConditions().size()).isEqualTo(totalConditions);
        assertThat(status.getConditions().stream().map(Condition::getStatus).collect(Collectors.toSet()))
                .isEqualTo(Set.of(conditionStatus));
        if (Valid.equals(validity)) {
            assertThat(status.getConditions().stream().map(Condition::getMessage))
                    .allMatch(msg -> msg.matches(
                            ".*localhost:[0-9]+/(consensus|backup|event|record|recordSidecar|otel):sha256:.*"));
        }

        Changesets specChangesets =
                networkOperation.getSpec().getNetworkUpgrade().getChangesets();
        SoftwareSourceStatus sourceStatus = status.getSoftwareSourceStatus();
        match(
                validity,
                sourceStatus.getConsensusNode(),
                specChangesets.getConsensusNodeChangeset().getSoftwareVersionSource());

        if (sourceStatus.getBackupUploader() != null) {
            match(
                    validity,
                    sourceStatus.getBackupUploader(),
                    specChangesets.getBackupUploaderChangeset().getSoftwareVersionSource());
        }

        if (sourceStatus.getEventsStreamUploader() != null) {
            match(
                    validity,
                    sourceStatus.getEventsStreamUploader(),
                    specChangesets.getEventsStreamUploaderChangeset().getSoftwareVersionSource());
        }

        if (sourceStatus.getRecordStreamSidecarUploader() != null) {
            match(
                    validity,
                    sourceStatus.getRecordStreamSidecarUploader(),
                    specChangesets.getRecordStreamSidecarUploaderChangeset().getSoftwareVersionSource());
        }

        if (sourceStatus.getRecordStreamUploader() != null) {
            match(
                    validity,
                    sourceStatus.getRecordStreamUploader(),
                    specChangesets.getRecordStreamUploaderChangeset().getSoftwareVersionSource());
        }

        if (sourceStatus.getOtelCollector() != null) {
            match(
                    validity,
                    sourceStatus.getOtelCollector(),
                    specChangesets.getOtelCollector().getSoftwareVersionSource());
        }
    }

    void match(
            final ImageRepositoryStatus.Validity validity,
            final List<ImageRepositoryStatus> statusList,
            final SoftwareVersionSource source) {
        assertThat(statusList.stream().map(ImageRepositoryStatus::getValidity).collect(Collectors.toSet()))
                .isEqualTo(Set.of(validity));
        assertThat(statusList.stream()
                        .map(ImageRepositoryStatus::getImagePullPolicy)
                        .map(ImageRepositoryStatus.ImagePullPolicy::getValue)
                        .collect(Collectors.toSet()))
                .isEqualTo(Set.of(source.getImagePullPolicy().getValue()));
        assertThat(source.getImageVerificationSpec())
                .containsAll(statusList.stream()
                        .map(ImageRepositoryStatus::getImageVerificationSpec)
                        .filter(Objects::nonNull)
                        .collect(Collectors.toSet()));
        for (ImageRepository repository : source.getImageRepositories()) {
            assertThat(statusList.stream())
                    .anyMatch(status -> Objects.equals(repository.getImageName(), status.getImageName())
                            && Objects.equals(repository.getImageTag(), status.getImageTag())
                            && Objects.equals(repository.getImagePullSecrets(), status.getImagePullSecrets()));
        }
    }

    SoftwareVersionSource softwareVersionSource(
            final String imageName,
            final String imageTag,
            final String os,
            final String architecture,
            final String... layerHashes) {
        String registry = "localhost:" + wm.httpsPort();
        SoftwareVersionSource softwareVersionSource = new SoftwareVersionSource();
        softwareVersionSource.setImageRepositories(List.of(imageRepository(registry, imageName, imageTag)));
        softwareVersionSource.setImageVerificationSpec(List.of(imageVerificationSpec(os, architecture, layerHashes)));
        return softwareVersionSource;
    }

    ImageRepository imageRepository(final String repository, final String imageName, final String imageTag) {
        return edit(new ImageRepository())
                .with(builder -> builder.withRepository(repository)
                        .withImageName(imageName)
                        .withImageTag(imageTag))
                .build();
    }

    ImageVerificationSpec imageVerificationSpec(
            final String os, final String architecture, final String... layerHashes) {
        return edit(new ImageVerificationSpec())
                .with(builder -> builder.withArchitecture(ImageVerificationSpec.Architecture.valueOf(architecture))
                        .withOs(ImageVerificationSpec.Os.valueOf(os))
                        .withLayerHashes(layerHashes))
                .build();
    }

    static class Ctx implements Context<NetworkOperation> {
        @Override
        public Optional<RetryInfo> getRetryInfo() {
            return Optional.empty();
        }

        @Override
        public <R> Set<R> getSecondaryResources(Class<R> aClass) {
            return Set.of();
        }

        @Override
        public <R> Optional<R> getSecondaryResource(Class<R> aClass, String s) {
            return Optional.empty();
        }

        @Override
        public ControllerConfiguration<NetworkOperation> getControllerConfiguration() {
            return null;
        }

        @Override
        public ManagedWorkflowAndDependentResourceContext managedWorkflowAndDependentResourceContext() {
            return null;
        }

        @Override
        public EventSourceRetriever<NetworkOperation> eventSourceRetriever() {
            return null;
        }

        @Override
        public KubernetesClient getClient() {
            return null;
        }

        @Override
        public ExecutorService getWorkflowExecutorService() {
            return null;
        }

        @Override
        public IndexedResourceCache<NetworkOperation> getPrimaryCache() {
            return null;
        }

        @Override
        public boolean isNextReconciliationImminent() {
            return false;
        }
    }
}
