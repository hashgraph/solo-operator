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

package com.hedera.solo.operator.test.setup;

import com.hedera.fullstack.helm.client.HelmClient;
import com.hedera.fullstack.helm.client.impl.DefaultHelmClientBuilder;
import com.hedera.fullstack.helm.client.model.Chart;
import com.hedera.fullstack.helm.client.model.Repository;
import com.hedera.fullstack.helm.client.model.install.InstallChartOptions;
import com.hedera.fullstack.helm.client.model.release.ReleaseItem;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.quarkus.arc.Lock;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.io.File;
import java.net.URL;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import org.jboss.logging.Logger;

@Lock
@ApplicationScoped
public class MinioSetupManager {
    private static boolean minioRepoAdded = false;
    private static boolean minioInstalled = false;

    @Inject
    Logger logger;

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient kubernetesClient;

    private HelmClient getHelmClient(String namespace) {
        // FIXME currently HelmClient does not let us set namespace for different operations
        // Therefore we need to create new instances for different namespaces.
        return new DefaultHelmClientBuilder()
                .defaultNamespace(namespace)
                .workingDirectory(new File(".").toPath())
                .build();
    }

    private void addMinioRepository(HelmClient helmClient) {
        if (minioRepoAdded) {
            return;
        }

        if (Objects.isNull(helmClient)) {
            helmClient = this.getHelmClient(Constants.DEFAULT_NAMESPACE);
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
        if (Objects.isNull(helmClient)) {
            helmClient = this.getHelmClient(Constants.DEFAULT_NAMESPACE);
        }

        final List<ReleaseItem> releases = helmClient.listReleases(true);
        for (ReleaseItem release : releases) {
            if (release.name().equals(releaseName)) {
                return true;
            }
        }

        return false;
    }

    private void installMinioOperator(HelmClient helmClient) {
        if (Objects.isNull(helmClient)) {
            helmClient = this.getHelmClient(Constants.MINIO_OPERATOR_NAMESPACE);
        }

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
                            1,
                            TimeUnit.MINUTES);

            logger.debug("installed MinIO operator");
        }

        logger.info("MinIO operator is ready");
    }

    private void installMinioTenant(HelmClient helmClient) {
        if (Objects.isNull(helmClient)) {
            helmClient = this.getHelmClient(Constants.MINIO_TENANT_NAMESPACE);
        }

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
                            2,
                            TimeUnit.MINUTES);

            logger.debug("installed MinIO tenant");
        }

        logger.info("MinIO tenant is ready");
    }

    public void install() {
        if (minioInstalled) {
            return;
        }

        final HelmClient helmClient = this.getHelmClient(Constants.MINIO_OPERATOR_NAMESPACE);
        this.addMinioRepository(helmClient);
        this.installMinioOperator(helmClient);
        this.installMinioTenant(null);

        minioInstalled = true;
    }

    public void uninstall() {
        if (!minioInstalled) {
            return;
        }

        HelmClient helmClient = this.getHelmClient(Constants.MINIO_TENANT_NAMESPACE);
        if (isChartInstalled(helmClient, Constants.MINIO_TENANT_RELEASE)) {
            helmClient.uninstallChart(Constants.MINIO_TENANT_RELEASE);
        }

        helmClient = this.getHelmClient(Constants.MINIO_OPERATOR_NAMESPACE);
        if (isChartInstalled(helmClient, Constants.MINIO_OPERATOR_RELEASE)) {
            helmClient.uninstallChart(Constants.MINIO_OPERATOR_RELEASE);
        }

        final List<String> namespaces = List.of(Constants.MINIO_OPERATOR_NAMESPACE, Constants.MINIO_TENANT_NAMESPACE);
        for (String namespace : namespaces) {
            final Namespace ns =
                    kubernetesClient.namespaces().withName(namespace).get();

            if (!Objects.isNull(ns)) {
                kubernetesClient.resource(ns).delete();
            }
        }

        minioInstalled = false;
    }
}
