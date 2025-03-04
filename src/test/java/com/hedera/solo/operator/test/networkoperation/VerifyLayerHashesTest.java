// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.networkoperation;

import static com.github.tomakehurst.wiremock.client.WireMock.equalTo;
import static com.github.tomakehurst.wiremock.client.WireMock.get;
import static com.github.tomakehurst.wiremock.client.WireMock.head;
import static com.github.tomakehurst.wiremock.client.WireMock.including;
import static com.github.tomakehurst.wiremock.client.WireMock.not;
import static com.github.tomakehurst.wiremock.client.WireMock.ok;
import static com.github.tomakehurst.wiremock.client.WireMock.unauthorized;
import static com.github.tomakehurst.wiremock.client.WireMock.urlMatching;
import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.data.AsciiArmor.base64Encode;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.Valid;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.selectedImage;
import static com.hedera.solo.operator.test.fixtures.networkoperation.TestVerifyLayerHashes.updateStatus;
import static java.nio.charset.StandardCharsets.UTF_8;
import static org.assertj.core.api.Assertions.assertThat;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.hedera.solo.operator.library.data.Digest;
import com.hedera.solo.operator.model.commonspec.ImageRepository;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.ImageVerificationSpec;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.LocalObjectReference;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.UUID;
import java.util.function.Function;
import java.util.stream.Collectors;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class VerifyLayerHashesTest {
    Namespace ns;
    String namespace;
    List<WireMockServer> wireMockServers = new ArrayList<>();

    @Inject
    KubernetesClient kubernetesClient;

    @BeforeEach
    public void setup() {
        ns = kubernetesClient
                .resource(edit(new Namespace())
                        .with(metadata -> metadata.editMetadata()
                                .withGenerateName(getClass().getSimpleName().toLowerCase() + "-"))
                        .build())
                .create();
        namespace = ns.getMetadata().getName();
    }

    @AfterEach
    public void cleanup() {
        kubernetesClient.namespaces().withName(namespace).delete();
        wireMockServers.forEach(WireMockServer::stop);
    }

    @Test
    void testMultipleSecrets() {
        WireMockServer wm1 = stubs(null, true, null, null);
        String registry1 = "localhost:" + wm1.port();
        WireMockServer wm2 = stubs(null, false, null, null, new Spec("linux", "amd64", "hash:1"));
        String registry2 = "localhost:" + wm2.port();
        WireMockServer wm3 = stubs(null, true, null, null, new Spec("linux", "arm64", "hash:0"));
        String registry3 = "localhost:" + wm3.port();
        WireMockServer wm4 = stubs(null, false, null, null, new Spec("linux", "arm64", "hash:0"));
        String registry4 = "localhost:" + wm4.port();

        Secret secret1 = createSecret(
                new AuthEntry(registry1, "/", "pass", BASE64_USER_PASS | PLAIN_USER_PASS),
                new AuthEntry(registry1 + "/abc", "abc", "abc-pass", PLAIN_USER_PASS));
        Secret secret2 = createSecret(
                new AuthEntry(registry1 + "/123", "123", "123-pass", BASE64_USER_PASS),
                new AuthEntry(registry1 + "/abc/123", "abc/123", "abc/123-pass", BASE64_USER_PASS | PLAIN_USER_PASS));

        SoftwareVersionSource source = new SoftwareVersionSource();
        source.setImageRepositories(new ArrayList<>(List.of(
                imageRepository(registry1, "", "", secret1, secret2),
                imageRepository(registry1, "abc", "", secret1, secret2),
                imageRepository(registry1, "123", "", secret1, secret2),
                imageRepository(registry1, "abc/123", "", secret1, secret2),
                imageRepository(registry1, "xyz", "", secret1, secret2),
                imageRepository(registry1, "abc-xyz", "", secret1, secret2),
                imageRepository(registry1, "123-xyz", "", secret1, secret2),
                imageRepository(registry1, "abc/123-xyz", "", secret1, secret2),
                imageRepository(registry2, "public", "1.0"),
                imageRepository(registry3, "public", "1.0"),
                imageRepository(registry4, "public", "1.0"))));
        source.setImageVerificationSpec(List.of(
                imageVerificationSpec("linux", "amd64", "hash:1"),
                imageVerificationSpec("linux", "arm64", "hash:2", "hash:3"),
                imageVerificationSpec("windows", "amd64", "hash:4")));

        verify(wm1, source, "/", "pass", 27, 17);
        verify(wm1, source, "abc", "abc-pass", 19, 9);
        verify(wm1, source, "123", "123-pass", 15, 5);
        verify(wm1, source, "abc/123", "abc/123-pass", 15, 5);
    }

    @Test
    void testSelection() {
        WireMockServer wm1 = stubs(
                null, true, null, null, new Spec("linux", "amd64", "hash:1"), new Spec("windows", "arm64", "hash:2"));
        String registry1 = "localhost:" + wm1.port();

        SoftwareVersionSource source = new SoftwareVersionSource();
        source.setImageRepositories(new ArrayList<>(
                List.of(imageRepository(registry1, "name", "1.0"), imageRepository(registry1, "name", "2.0"))));
        source.setImageVerificationSpec(List.of(imageVerificationSpec("linux", null, "hash:1")));

        // verify
        HasMetadata resource = resource();
        NetworkOperationStatus status = updateStatus(
                resource,
                null,
                kubernetesClient,
                source,
                condition("verify"),
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);
        assertThat(status.getSoftwareSourceStatus().getConsensusNode().stream()
                        .filter(ImageRepositoryStatus::getSelectedForDeployment)
                        .count())
                .isEqualTo(1);
        final String selectedImage =
                selectedImage(status.getSoftwareSourceStatus().getConsensusNode());
        assertThat(selectedImage).isNotNull();
        condition(status, resource, "verify");

        // re-verify
        resource = resource();
        status = updateStatus(
                resource,
                status,
                kubernetesClient,
                source,
                condition("re-verify"),
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);
        assertThat(selectedImage(status.getSoftwareSourceStatus().getConsensusNode()))
                .isEqualTo(selectedImage);
        condition(status, resource, "re-verify");

        // force re-selection
        resource = resource();
        source.setImageVerificationSpec(List.of(imageVerificationSpec(null, "arm64", "hash:2")));
        status = updateStatus(
                resource,
                status,
                kubernetesClient,
                source,
                condition("re-select"),
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);
        assertThat(selectedImage(status.getSoftwareSourceStatus().getConsensusNode()))
                .isNotNull();
        assertThat(selectedImage(status.getSoftwareSourceStatus().getConsensusNode()))
                .isNotEqualTo(selectedImage);
        condition(status, resource, "re-select");

        // force re-selection
        resource = resource();
        source.setImageVerificationSpec(List.of(imageVerificationSpec(null, null, "hash:1")));
        status = updateStatus(
                resource,
                status,
                kubernetesClient,
                source,
                condition("re-select"),
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);
        assertThat(selectedImage(status.getSoftwareSourceStatus().getConsensusNode()))
                .isNotNull();
        assertThat(selectedImage(status.getSoftwareSourceStatus().getConsensusNode()))
                .isEqualTo(selectedImage);
        condition(status, resource, "re-select");
    }

    ImageRepository imageRepository(
            final String repository, final String name, final String tag, final Secret... secrets) {
        return edit(new ImageRepository())
                .with(builder -> builder.withRepository(repository)
                        .withImageName(name)
                        .withImageTag(tag)
                        .withImagePullSecrets(Arrays.stream(secrets)
                                .map(Secret::getMetadata)
                                .map(ObjectMeta::getName)
                                .map(LocalObjectReference::new)
                                .toList()))
                .build();
    }

    ImageVerificationSpec imageVerificationSpec(
            final String os, final String architecture, final String... layerHashes) {
        ImageVerificationSpec spec = new ImageVerificationSpec();
        if (os != null) {
            spec.setOs(ImageVerificationSpec.Os.valueOf(os));
        }
        if (architecture != null) {
            spec.setArchitecture(ImageVerificationSpec.Architecture.valueOf(architecture));
        }
        spec.setLayerHashes(new ArrayList<>(Arrays.asList(layerHashes)));
        return spec;
    }

    record Spec(String os, String architecture, String... layerHashes) {}

    WireMockServer stubs(
            final WireMockServer wireMockServer,
            final boolean index,
            final String username,
            final String password,
            final Spec... specs) {
        WireMockServer wm = wireMockServer;
        if (wm == null) {
            wm = new WireMockServer(0);
            wm.start();
            wireMockServers.add(wm);
        }

        String token = UUID.randomUUID().toString();
        StringBuilder sb = new StringBuilder();
        wm.resetMappings();
        wm.stubFor(get("/v2/")
                .willReturn(unauthorized()
                        .withHeader(
                                "www-authenticate",
                                sb.append("Bearer realm=\"http://localhost:")
                                        .append(wm.port())
                                        .append("/v2/token\"")
                                        .toString())));
        if (username != null && password != null) {
            String auth = base64Encode(username + ":" + password);
            wm.stubFor(get(urlMatching("/v2/token.*"))
                    .withHeader("Authorization", equalTo("Basic " + auth))
                    .willReturn(ok().withHeader("Content-Type", "application/json")
                            .withBody("{\"token\":\"" + token + "\"}")));
            wm.stubFor(get(urlMatching("/v2/token.*"))
                    .withHeader("Authorization", not(equalTo("Basic " + auth)))
                    .willReturn(unauthorized()));
        } else {
            wm.stubFor(get(urlMatching("/v2/token.*"))
                    .willReturn(ok().withHeader("Content-Type", "application/json")
                            .withBody("{\"token\":\"" + token + "\"}")));
        }
        sb.setLength(0);
        sb.append("{\"manifests\":[");
        boolean first = true;
        for (Spec spec : specs) {
            String body = Arrays.stream(spec.layerHashes)
                    .map(layerHash -> "{\"mediaType\": \"application/vnd.oci.image.layer.v1.tar+gzip\",\"digest\":\""
                            + layerHash + "\"}")
                    .collect(Collectors.joining(
                            ",",
                            "{\"layers\":[",
                            "], \"annotations\": {\"port\": \"" + wm.port() + "\", \"os\": \"" + spec.os
                                    + "\", \"architecture\": \"" + spec.architecture + "\"}}"));
            String digest = "sha256:" + Digest.SHA_256.hexOf(body.getBytes(UTF_8));
            wm.stubFor(get(urlMatching("/v2(/.*)?/manifests/" + digest))
                    .withHeader("Accept", including("application/vnd.oci.image.manifest.v1+json"))
                    .withHeader("Authorization", equalTo("Bearer " + token))
                    .willReturn(ok().withHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json")
                            .withBody(body)));
            if (!index) {
                wm.stubFor(get(urlMatching("/v2(/.*)?/manifests/(?!sha256:[0-9a-fA-F]{64}).*"))
                        .withHeader("Accept", including("application/vnd.oci.image.manifest.v1+json"))
                        .withHeader("Authorization", equalTo("Bearer " + token))
                        .willReturn(ok().withHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json")
                                .withBody(body)));
            }
            for (String layerHash : spec.layerHashes) {
                wm.stubFor(head(urlMatching("/v2(/.*)?/blobs/" + layerHash))
                        .withHeader("Authorization", equalTo("Bearer " + token))
                        .willReturn(ok()));
            }
            if (first) {
                first = false;
            } else {
                sb.append(",");
            }
            sb.append("{\"mediaType\":\"application/vnd.oci.image.manifest.v1+json\",\"digest\":\"")
                    .append(digest)
                    .append("\",\"platform\":{\"os\":\"")
                    .append(spec.os)
                    .append("\",\"architecture\":\"")
                    .append(spec.architecture)
                    .append("\"}}");
        }
        if (index) {
            wm.stubFor(get(urlMatching("/v2(/.*)?/manifests/(?!sha256:[0-9a-fA-F]{64}).*"))
                    .withHeader("Accept", including("application/vnd.oci.image.index.v1+json"))
                    .withHeader("Authorization", equalTo("Bearer " + token))
                    .willReturn(ok().withHeader("Content-Type", "application/vnd.oci.image.index.v1+json")
                            .withBody(sb.append("]}").toString())));
        }
        return wm;
    }

    static final int PLAIN_USER_PASS = 1;
    static final int BASE64_USER_PASS = 2;

    record AuthEntry(String key, String username, String password, int type) {}

    String authfile(final AuthEntry... entries) {
        StringBuilder sb = new StringBuilder("{\"auths\":{");
        boolean first = true;
        for (AuthEntry entry : entries) {
            boolean plain = (entry.type & PLAIN_USER_PASS) != 0;
            boolean base64 = (entry.type & BASE64_USER_PASS) != 0;
            if (first) {
                first = false;
            } else {
                sb.append(",");
            }
            sb.append("\"").append(entry.key).append("\":{");
            if (plain) {
                sb.append("\"username\":\"").append(entry.username).append("\",");
                sb.append("\"password\":\"").append(entry.password).append("\"");
                if (base64) {
                    sb.append(",");
                }
            }
            if (base64) {
                sb.append("\"auth\":\"")
                        .append(base64Encode(entry.username + ":" + entry.password))
                        .append("\"");
            }
            sb.append("}");
        }
        sb.append("}}");
        return sb.toString();
    }

    void verify(
            WireMockServer wm,
            SoftwareVersionSource source,
            String username,
            String password,
            int totalCount,
            long validCount) {
        stubs(
                wm,
                true,
                username,
                password,
                new Spec("linux", "amd64", "hash:1"),
                new Spec("linux", "arm64", "hash:2", "hash:3"),
                new Spec("windows", "amd64", "hash:0"),
                new Spec("windows", "arm64", "hash:0"));

        HasMetadata resource = resource();
        String message = UUID.randomUUID().toString();
        NetworkOperationStatus status = updateStatus(
                resource,
                null,
                kubernetesClient,
                source,
                condition(message),
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);
        List<ImageRepositoryStatus> statusList =
                status.getSoftwareSourceStatus().getConsensusNode();
        assertThat(statusList).hasSize(totalCount);
        assertThat(statusList.stream()
                        .filter(ImageRepositoryStatus::getSelectedForDeployment)
                        .count())
                .isEqualTo(1);
        assertThat(statusList.stream()
                        .filter(s -> Valid.equals(s.getValidity()))
                        .count())
                .isEqualTo(validCount);
        condition(status, resource, message);
    }

    Function<NetworkOperationStatus, Condition> condition(String message) {
        return status -> new Condition(null, message, null, "reason", "status", "type");
    }

    Secret createSecret(final AuthEntry... entries) {
        return createSecret(authfile(entries));
    }

    Secret createSecret(final String authfile) {
        return kubernetesClient
                .resource(edit(new Secret())
                        .with(metadata -> metadata.editMetadata()
                                .withGenerateName("secret-")
                                .withNamespace(namespace))
                        .with(data -> data.addToData(".dockerconfigjson", base64Encode(authfile)))
                        .build())
                .create();
    }

    HasMetadata resource() {
        return edit(ns).with(metadata ->
                        metadata.editMetadata().withNamespace(namespace).withGeneration(System.nanoTime()))
                .build();
    }

    void condition(NetworkOperationStatus status, HasMetadata resource, String message) {
        Condition condition = status.getConditions().stream()
                .filter(cond -> "type".equals(cond.getType()))
                .findFirst()
                .orElse(null);
        assertThat(condition).isNotNull();
        assertThat(condition.getMessage()).isEqualTo(message);
        assertThat(condition.getObservedGeneration())
                .isEqualTo(resource.getMetadata().getGeneration());
    }
}
