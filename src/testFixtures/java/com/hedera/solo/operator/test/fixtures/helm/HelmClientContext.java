// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.helm;

import static com.hedera.solo.operator.test.fixtures.kubernetes.KubeConfig.kubeConfig;
import static java.nio.file.attribute.PosixFilePermission.OWNER_READ;
import static java.nio.file.attribute.PosixFilePermission.OWNER_WRITE;

import com.hedera.fullstack.helm.client.HelmClient;
import com.hedera.fullstack.helm.client.HelmClientBuilder;
import com.hedera.fullstack.helm.client.impl.DefaultHelmClient;
import com.hedera.fullstack.helm.client.impl.DefaultHelmClientBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.io.File;
import java.io.IOException;
import java.lang.reflect.Field;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Comparator;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Stream;

/**
 * The {@code HelmClientContext} class provides a context for managing a Helm client with a temporary kubeconfig file.
 * It implements {@link AutoCloseable} to ensure proper cleanup of resources when no longer needed.
 */
public class HelmClientContext implements AutoCloseable {
    public final HelmClient helmClient;
    private final Path kubeConfig;

    /**
     * Constructs a HelmClientContext with a temporary kubeconfig file and a Helm client.
     *
     * @param kubernetesClient The Kubernetes client to interact with the cluster.
     * @param configure A function to configure the HelmClientBuilder.
     * @throws IOException If an error occurs while creating the kubeconfig file.
     */
    public HelmClientContext(
            final KubernetesClient kubernetesClient, final Function<HelmClientBuilder, HelmClientBuilder> configure)
            throws IOException {
        kubeConfig = File.createTempFile("kube-config-", ".yaml").toPath();
        Files.setPosixFilePermissions(kubeConfig, Set.of(OWNER_READ, OWNER_WRITE));
        Files.writeString(kubeConfig, kubeConfig(kubernetesClient));
        helmClient = configure
                .apply(new DefaultHelmClientBuilder().kubeConfig(kubeConfig))
                .build();
    }

    @Override
    public void close() throws Exception {
        if (helmClient instanceof DefaultHelmClient defaultHelmClient) {
            Field helmExecutable = DefaultHelmClient.class.getDeclaredField("helmExecutable");
            helmExecutable.setAccessible(true);
            Object object = helmExecutable.get(defaultHelmClient);
            if (object instanceof Path path) {
                try (Stream<Path> paths = Files.walk(path.getParent())) {
                    paths.sorted(Comparator.reverseOrder()).map(Path::toFile).forEach(File::delete);
                }
            }
        }
        kubeConfig.toFile().delete();
    }
}
