// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.setup;

import static java.nio.file.attribute.PosixFilePermission.OWNER_READ;
import static java.nio.file.attribute.PosixFilePermission.OWNER_WRITE;

import com.hedera.fullstack.helm.client.HelmClient;
import com.hedera.fullstack.helm.client.impl.DefaultHelmClient;
import com.hedera.fullstack.helm.client.impl.DefaultHelmClientBuilder;
import com.hedera.fullstack.helm.client.model.Chart;
import com.hedera.fullstack.helm.client.model.Repository;
import com.hedera.fullstack.helm.client.model.install.InstallChartOptions;
import com.hedera.fullstack.helm.client.model.release.ReleaseItem;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.client.Config;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.quarkus.arc.Lock;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.io.File;
import java.lang.reflect.Field;
import java.net.URL;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.stream.Stream;
import org.jboss.logging.Logger;

@Lock(time = 5, unit = TimeUnit.MINUTES) // this lock is needed when tests are run in parallel
@ApplicationScoped
public class MinioSetupManager {
    private static boolean minioRepoAdded = false;
    private static boolean minioInstalled = false;

    @Inject
    Logger logger;

    @Inject
    KubernetesClient kubernetesClient;

    private Path kubeConfigFile = null;

    /**
     * Set up a temporary kubeconfig file This is only needed to support testcontainer
     *
     * @return Path to temporary kube configuration file
     */
    private Path setupKubeConfig() {
        if (Objects.nonNull(kubeConfigFile)) {
            return kubeConfigFile;
        }

        Config config = kubernetesClient.getConfiguration();
        final String kubeConfig =
                """
                        apiVersion: v1
                        clusters:
                        - cluster:
                            certificate-authority%s
                            server: %s
                          name: default
                        contexts:
                        - context:
                            cluster: default
                            user: default
                          name: default
                        current-context: default
                        kind: Config
                        preferences: {}
                        users:
                        - name: default
                          user:
                            client-certificate%s
                            client-key%s
                        """
                        .formatted(
                                null != config.getCaCertData()
                                        ? "-data: " + config.getCaCertData()
                                        : ": " + config.getCaCertFile(),
                                config.getMasterUrl(),
                                null != config.getClientCertData()
                                        ? "-data: " + config.getClientCertData()
                                        : ": " + config.getClientCertFile(),
                                null != config.getClientKeyData()
                                        ? "-data: " + config.getClientKeyData()
                                        : ": " + config.getClientKeyFile());

        try {
            kubeConfigFile = File.createTempFile("kube-", ".config").toPath();
            kubeConfigFile.toFile().deleteOnExit();
            Files.setPosixFilePermissions(kubeConfigFile, Set.of(OWNER_READ, OWNER_WRITE));
            Files.writeString(kubeConfigFile, kubeConfig);
            return kubeConfigFile;
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    private HelmClient getHelmClient(String namespace) {
        // FIXME currently HelmClient does not let us set namespace for different operations
        // Therefore we need to create new instances for different namespaces.

        return new DefaultHelmClientBuilder()
                .kubeConfig(this.setupKubeConfig())
                .defaultNamespace(namespace)
                .workingDirectory(new File(".").toPath())
                .build();
    }

    private void addMinioRepository(HelmClient helmClient) {
        Objects.requireNonNull(helmClient, "helmClient is null");

        if (minioRepoAdded) {
            return;
        }

        final List<Repository> repositories = helmClient.listRepositories();
        for (Repository repository : repositories) {
            if (repository.name().equals(Constants.MINIO_CHART_REPO)) {
                minioRepoAdded = true;
                return;
            }
        }

        helmClient.addRepository(new Repository(Constants.MINIO_CHART_REPO, Constants.MINIO_CHART_REPO_URL));
        minioRepoAdded = true;
    }

    private boolean isChartInstalled(HelmClient helmClient, String releaseName) {
        Objects.requireNonNull(helmClient, "helmClient is null");

        final List<ReleaseItem> releases = helmClient.listReleases(true);
        for (ReleaseItem release : releases) {
            if (release.name().equals(releaseName)) {
                return true;
            }
        }

        return false;
    }

    private void installMinioOperator(HelmClient helmClient) {
        Objects.requireNonNull(helmClient, "helmClient is null");

        if (!isChartInstalled(helmClient, Constants.MINIO_OPERATOR_RELEASE)) {
            helmClient.installChart(
                    Constants.MINIO_OPERATOR_RELEASE,
                    new Chart(Constants.MINIO_OPERATOR_CHART),
                    InstallChartOptions.builder().createNamespace(true).build());

            kubernetesClient
                    .pods()
                    .inNamespace(Constants.MINIO_OPERATOR_NAMESPACE)
                    .withLabels(Map.of(Constants.MINIO_OPERATOR_LABEL, Constants.MINIO_OPERATOR_RELEASE))
                    .waitUntilCondition(
                            pod -> Objects.nonNull(pod)
                                    && pod.getStatus().getContainerStatuses().stream()
                                            .allMatch(containerStatus ->
                                                    containerStatus.getReady() && containerStatus.getStarted()),
                            5,
                            TimeUnit.MINUTES);

            logger.debug("installed MinIO operator");
        }

        logger.info("MinIO operator is ready");
    }

    private void installMinioTenant(HelmClient helmClient) {
        Objects.requireNonNull(helmClient, "helmClient is null");

        if (!isChartInstalled(helmClient, Constants.MINIO_TENANT_RELEASE)) {
            final URL valuesFile = getClass().getClassLoader().getResource("minio-values.yaml");
            if (Objects.isNull(valuesFile)) {
                throw new IllegalArgumentException("minio-values.yaml not found");
            }

            helmClient.installChart(
                    Constants.MINIO_TENANT_RELEASE,
                    new Chart(Constants.MINIO_TENANT_CHART),
                    InstallChartOptions.builder()
                            .values(List.of(valuesFile.getPath()))
                            .createNamespace(true)
                            .build());

            kubernetesClient
                    .pods()
                    .inNamespace(Constants.MINIO_TENANT_NAMESPACE)
                    .withLabel(Constants.MINIO_TENANT_LABEL, Constants.MINIO_TENANT_RELEASE)
                    .waitUntilCondition(
                            pod -> Objects.nonNull(pod)
                                    && pod.getStatus().getContainerStatuses().stream()
                                            .allMatch(containerStatus ->
                                                    containerStatus.getReady() && containerStatus.getStarted()),
                            5,
                            TimeUnit.MINUTES);

            logger.debug("installed MinIO tenant");
        }

        logger.info("MinIO tenant is ready");
    }

    // workaround for https://github.com/hashgraph/full-stack-testing/issues/964
    public void cleanup(final HelmClient helmClient) {
        try {
            if (helmClient instanceof DefaultHelmClient defaultHelmClient) {
                Field helmExecutable = DefaultHelmClient.class.getDeclaredField("helmExecutable");
                helmExecutable.setAccessible(true);
                Object object = helmExecutable.get(defaultHelmClient);
                if (object instanceof Path path) {
                    try (Stream<Path> paths = Files.walk(path.getParent())) {
                        paths.sorted(Comparator.reverseOrder())
                                .map(Path::toFile)
                                .forEach(File::delete);
                    }
                }
            }
        } catch (Exception ignored) {
        }
    }

    public void install() {
        if (minioInstalled) {
            return;
        }

        HelmClient helmClient = null;
        try {
            helmClient = this.getHelmClient(Constants.MINIO_OPERATOR_NAMESPACE);
            this.addMinioRepository(helmClient);
            this.installMinioOperator(helmClient);
        } finally {
            cleanup(helmClient);
        }

        helmClient = null;
        try {
            helmClient = this.getHelmClient(Constants.MINIO_TENANT_NAMESPACE);
            this.installMinioTenant(helmClient);
        } finally {
            cleanup(helmClient);
        }

        minioInstalled = true;
    }

    public void uninstall() {
        if (!minioInstalled) {
            return;
        }

        HelmClient helmClient = null;
        try {
            helmClient = this.getHelmClient(Constants.MINIO_TENANT_NAMESPACE);
            if (isChartInstalled(helmClient, Constants.MINIO_TENANT_RELEASE)) {
                helmClient.uninstallChart(Constants.MINIO_TENANT_RELEASE);
            }
        } finally {
            cleanup(helmClient);
        }

        helmClient = null;
        try {
            helmClient = this.getHelmClient(Constants.MINIO_OPERATOR_NAMESPACE);
            if (isChartInstalled(helmClient, Constants.MINIO_OPERATOR_RELEASE)) {
                helmClient.uninstallChart(Constants.MINIO_OPERATOR_RELEASE);
            }
        } finally {
            cleanup(helmClient);
        }

        minioInstalled = false;
    }
}
