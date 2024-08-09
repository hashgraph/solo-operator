/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.test.capsule;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.fullstack.helm.client.HelmClient;
import com.hedera.fullstack.helm.client.impl.DefaultHelmClientBuilder;
import com.hedera.fullstack.helm.client.model.Chart;
import com.hedera.fullstack.helm.client.model.Repository;
import com.hedera.fullstack.helm.client.model.install.InstallChartOptions;
import com.hedera.fullstack.helm.client.model.release.ReleaseItem;
import com.hedera.hashgraph.sdk.Client;
import com.hedera.hashgraph.sdk.FileContentsQuery;
import com.hedera.hashgraph.sdk.FileId;
import com.hedera.hashgraph.sdk.NodeAddress;
import com.hedera.hashgraph.sdk.NodeAddressBook;
import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.commonspec.StreamUploaderBuilder;
import com.hedera.solo.operator.model.commonspec.streamuploader.ConfigBuilder;
import com.hedera.solo.operator.model.commonspec.streamuploader.config.Gcs;
import com.hedera.solo.operator.model.commonspec.streamuploader.config.GcsBuilder;
import com.hedera.solo.operator.model.commonspec.streamuploader.config.S3;
import com.hedera.solo.operator.model.commonspec.streamuploader.config.S3Builder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Sidecars;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.SidecarsBuilder;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.test.Environment;
import com.hedera.solo.operator.test.HederaSdkClientFactory;
import com.hedera.solo.operator.test.fixtures.Resources;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import jakarta.inject.Inject;

import java.io.File;
import java.lang.invoke.MethodHandles;
import java.net.URL;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public abstract class ConsensusCapsuleTestAbstract {
    protected String namespace;

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient kubernetesClient;

    @Inject
    protected Environment environment;

    @Inject
    protected Operator operator;


    public ConsensusCapsuleTestAbstract() {
        setNamespace(MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase());
    }

    protected void setNamespace(String namespace) {
        this.namespace = namespace;
    }

    @BeforeEach
    void setup() {
        if (!environment.isDevModeActive()) {
            operator.start();
        }

        cleanup();

        this.setupMinio();

        // create orbit
        createOrbit();
    }

    @AfterEach
    void cleanup() {
        kubernetesClient
                .resources(NetworkInitialization.class)
                .inNamespace(namespace)
                .delete();

        kubernetesClient
                .resources(ConsensusCapsule.class)
                .inNamespace(namespace)
                .delete();

        Resources.removeByName(kubernetesClient, Orbit.class, namespace);

        await().atMost(Duration.ofSeconds(30))
                .until(() -> kubernetesClient.namespaces().withName(namespace).get() == null);
    }

    record NodeInfo(String name, String accountId, int weight, String internal, String external) {
    }

    ConfigMap prepareClusterConfig(ConfigMap configMap, List<NodeInfo> nodes) {
        // should probably set the chainId in bootstrap.properties
        final Optional<String> cid = configMap
                .getData()
                .get("bootstrap.properties")
                .lines()
                .filter(s -> s.startsWith("contracts.chainId="))
                .findFirst();

        if (cid.isEmpty()) {
            throw new RuntimeException("contacts.chainid property not found");
        }

        String chainId = cid.get().substring(18);

        int nodeId = 0;
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(chainId).append("\n");
        sb.append("app, HederaNode.jar\n");
        for (NodeInfo node : nodes) {
            sb.append("address, %d, %s, %s, 1, %s, 50111, %s, 50111, 0.0.%d\n"
                    .formatted(nodeId, node.name, node.name, node.internal, node.external, nodeId + 3));
            nodeId++;
        }
        sb.append("nextNodeId, ").append(nodeId);
        return configMap.edit().addToData("config.txt", sb.toString()).build();
    }

    private void setupMinio() {

        final HelmClient helmClient = new DefaultHelmClientBuilder()
                .defaultNamespace("minio")
                .workingDirectory(new File(".").toPath())
                .build();
        final URL valuesFile = getClass().getClassLoader().getResource("minio-values.yaml");
        assert valuesFile != null;


        final List<Repository> repositories = helmClient.listRepositories();
        boolean hasRepository = false;
        for (Repository repository : repositories) {
            if (repository.name().equals(Constants.MINIO_CHART_REPO)) {
                hasRepository = true;
                break;
            }
        }

        if (!hasRepository) {
            helmClient.addRepository(new Repository(Constants.MINIO_CHART_REPO, Constants.MINIO_CHART_REPO_URL));
        }

        final List<ReleaseItem> releases = helmClient.listReleases(true);
        boolean hasMinioOperator = false;
        boolean hasMinioTenant = false;
        for (ReleaseItem release : releases) {
            if (release.name().equals(Constants.MINIO_OPERATOR_RELEASE)) {
                hasMinioOperator = true;
            } else if (release.name().equals(Constants.MINIO_TENANT_RELEASE)) {
                hasMinioTenant = true;
            }
        }

        if (!hasMinioOperator) {
            helmClient.installChart(Constants.MINIO_OPERATOR_RELEASE,
                    new Chart(Constants.MINIO_OPERATOR_CHART),
                    InstallChartOptions.builder()
                            .createNamespace(true).build());
        }

        if (!hasMinioTenant) {
            helmClient.installChart(Constants.MINIO_TENANT_RELEASE,
                    new Chart(Constants.MINIO_TENANT_CHART),
                    InstallChartOptions.builder()
                            .values(List.of(valuesFile.getPath()))
                            .createNamespace(true).build());
        }

    }

    private List<NodeInfo> prepareNodes(List<String> nodeNames) {
        List<NodeInfo> nodes = new ArrayList<>();
        int accStart = Constants.HEDERA_ACCOUNT_START;
        for (String nodeName : nodeNames) {
            nodes.add(new NodeInfo(
                    nodeName,
                    String.format("0.0.%d", accStart++),
                    1,
                    String.format("%s.consensus.%s.svc.cluster.local", nodeName, namespace),
                    String.format("%s-svc.%s.svc.cluster.local", nodeName, namespace)
            ));
        }

        return nodes;
    }

    private void setupConfigMaps(List<NodeInfo> nodes) {
        // create cluster-config configmap
        kubernetesClient.resource(prepareClusterConfig(
                        loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                                .edit()
                                .editMetadata()
                                .withNamespace(namespace)
                                .endMetadata()
                                .build(),
                        nodes))
                .create();

        // create other config-maps
        for (String cmFile : List.of("public-keys.yaml")) {
            kubernetesClient
                    .resource(loadYaml(ConfigMap.class, getClass(), cmFile)
                            .edit()
                            .editMetadata()
                            .withNamespace(namespace)
                            .endMetadata()
                            .build())
                    .create();
        }
    }

    private void setupSecrets(List<NodeInfo> nodes) {
        final ArrayList<String> secretFiles = new ArrayList<>(List.of(
                "uploader-secrets.yaml",
                "mirror-secrets.yaml"
        ));

        for (NodeInfo node : nodes) {
            secretFiles.add(String.format("%s-keys.yaml", node.name));
        }

        for (String sf : secretFiles) {
            kubernetesClient
                    .resource(loadYaml(Secret.class, getClass(), sf)
                            .edit()
                            .editMetadata()
                            .withNamespace(namespace)
                            .endMetadata()
                            .build())
                    .create();

        }
    }

    @Test
    void testSingleNodeDeployment() {
        final List<String> nodeNames = List.of("node0");
        final List<NodeInfo> nodes = prepareNodes(nodeNames);
        this.setupConfigMaps(nodes);
        this.setupSecrets(nodes);

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(nodes);

        // create network initialization
        createNetworkInitialization(consensusCapsules);

        // test with client
        await().atMost(90, TimeUnit.SECONDS).untilAsserted(() -> {
            HederaSdkClientFactory sdkClientFactory = new HederaSdkClientFactory(kubernetesClient, namespace);
            checkAddressBook(sdkClientFactory.directContextCreator("node0"), 1);
        });
    }

    @Test
    void testMultiNodeDeployment() throws Exception {
        final List<String> nodeNames = List.of("node0", "node1", "node2");
        final List<NodeInfo> nodes = prepareNodes(nodeNames);
        this.setupConfigMaps(nodes);
        this.setupSecrets(nodes);

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(nodes);

        // create network initialization
        createNetworkInitialization(consensusCapsules);

        // test with client
        await().atMost(90, TimeUnit.SECONDS).untilAsserted(() -> {
            HederaSdkClientFactory sdkClientFactory = new HederaSdkClientFactory(kubernetesClient, namespace);
            checkAddressBook(sdkClientFactory.directContextCreator(nodeNames.getFirst()), 3);
        });
    }

    void createOrbit() {
        Orbit orbit = new OrbitBuilder()
                .withNewMetadata()
                .withName(namespace)
                .endMetadata()
                .withNewSpec()
                .endSpec()
                .build();

        kubernetesClient.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() ->
                assertThat(kubernetesClient.resource(orbit).get()).isNotNull());
    }

    S3 s3Config = new S3Builder()
            .withEnabled()
            .withUrl("http://minio-tenant-hl.minio.svc.cluster.local:9000")
            .build();
    StreamUploader recordStreamUploader = new StreamUploaderBuilder()
            .withEnabled()
            .withConfig(new ConfigBuilder()
                    .withDebug()
                    .withCompression()
                    .withS3(s3Config)
                    .withGcs(new GcsBuilder().withEnabled(false).build())
                    .build())
            .build();
    Sidecars sidecars = new SidecarsBuilder()
            .withRecordStreamUploader(recordStreamUploader)
            .build();

    List<ConsensusCapsule> createConsensusCapsule(List<NodeInfo> nodes) {
        List<ConsensusCapsule> list = new ArrayList<>();
        for (NodeInfo node : nodes) {
            final ConsensusCapsule consensusCapsule = new ConsensusCapsuleBuilder()
                    .withNewMetadata()
                    .withName(node.name)
                    .withNamespace(namespace)
                    .withLabels(Map.of("account-id", node.accountId))
                    .endMetadata()
                    .withNewSpec()
                    .withSidecars(sidecars)
                    .endSpec()
                    .build();
            list.add(kubernetesClient.resource(consensusCapsule).create());
        }

        for (ConsensusCapsule consensusCapsule : list) {
            String name = consensusCapsule.getMetadata().getName();
            await().atMost(30999, TimeUnit.SECONDS).untilAsserted(() -> {
                ConsensusCapsule capsule =
                        kubernetesClient.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                Secret secret = kubernetesClient
                        .secrets()
                        .inNamespace(namespace)
                        .withName(name + "-keys")
                        .get();
                assertThat(secret).isNotNull();
                Service service = kubernetesClient
                        .services()
                        .inNamespace(namespace)
                        .withName(name + "-svc")
                        .get();
                assertThat(service).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isFalse();
            });
        }
        return list;
    }

    void createNetworkInitialization(List<ConsensusCapsule> consensusCapsules) {
        final NetworkInitialization networkInitialization = new NetworkInitializationBuilder()
                .withNewMetadata()
                .withName("network-initialization")
                .withNamespace(namespace)
                .endMetadata()
                .withNewSpec()
                .withMode(NetworkInitializationSpec.Mode.FirstInitialization)
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        kubernetesClient.resource(networkInitialization).create();

        await().atMost(90, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkInitialization netInit =
                    kubernetesClient.resource(networkInitialization).get();
            assertThat(netInit).isNotNull();
            for (ConsensusCapsule consensusCapsule : consensusCapsules) {
                String name = consensusCapsule.getMetadata().getName();
                ConsensusCapsule capsule =
                        kubernetesClient.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isTrue();

                Deployment deployment = kubernetesClient
                        .resources(Deployment.class)
                        .inNamespace(namespace)
                        .withName(name + "-deploy")
                        .get();
                assertThat(deployment).isNotNull();
                assertThat(deployment.getStatus()).isNotNull();
                assertThat(deployment.getStatus().getAvailableReplicas()).isEqualTo(1);

                assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
                assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Succeeded);
            }
        });
    }

    List<NodeAddress> addresses(final Client client) throws Exception {
        return NodeAddressBook.fromBytes(
                        new FileContentsQuery().setFileId(FileId.ADDRESS_BOOK).execute(client))
                .getNodeAddresses();
    }

    void checkAddressBook(
            final Supplier<HederaSdkClientFactory.ClientContext> clientContextSupplier, final int expectedAddresses)
            throws Exception {

        try (HederaSdkClientFactory.ClientContext context = clientContextSupplier.get()) {
            context.client.setOperator(Constants.OPERATOR_ACCOUNT_ID, Constants.OPERATOR_PRIVATE_KEY);
            context.client.setTransportSecurity(false);
            assertThat(addresses(context.client).size()).isEqualTo(expectedAddresses);
            context.client.setTransportSecurity(true).setVerifyCertificates(true);
            assertThat(addresses(context.client).size()).isEqualTo(expectedAddresses);
        }
    }
}
