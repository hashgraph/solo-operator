// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.microcks;

import static org.awaitility.Awaitility.await;

import com.hedera.fullstack.helm.client.HelmClient;
import com.hedera.fullstack.helm.client.model.Chart;
import com.hedera.fullstack.helm.client.model.Repository;
import com.hedera.fullstack.helm.client.model.install.InstallChartOptions;
import com.hedera.solo.operator.test.fixtures.helm.HelmClientContext;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.LocalPortForward;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.PrintStream;
import java.net.URI;
import java.net.URL;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.time.Duration;
import java.util.List;
import java.util.UUID;

/**
 * MicrocksService is a utility class that manages the deployment and interaction with the Microcks service using Helm
 * and Kubernetes.
 */
public class MicrocksService implements AutoCloseable {
    private final HelmClientContext helmClientContext;
    private final KubernetesClient kubernetesClient;
    private final String namespace;
    private boolean ready = false;

    /**
     * Constructs a MicrocksService instance and deploys the Microcks service using Helm.
     *
     * @param kubernetesClient The Kubernetes client to interact with the cluster.
     * @param namespace The namespace where the Microcks service will be deployed.
     * @throws IOException If an error occurs during the Helm chart installation.
     */
    public MicrocksService(final KubernetesClient kubernetesClient, final String namespace) throws IOException {
        helmClientContext = new HelmClientContext(kubernetesClient, builder -> builder.defaultNamespace(namespace));
        HelmClient helmClient = helmClientContext.helmClient;
        helmClient.addRepository(new Repository("microcks", "https://microcks.io/helm"));
        helmClient.installChart(
                "microcks",
                new Chart("microcks/microcks"),
                InstallChartOptions.builder()
                        .version("1.9.1")
                        .createNamespace(true)
                        .set(List.of("microcks.grpcEnableTLS=false", "keycloak.enabled=false"))
                        .build());
        this.kubernetesClient = kubernetesClient;
        this.namespace = namespace;
    }

    /**
     * Gets the namespace where the Microcks service is deployed.
     *
     * @return The namespace as a string
     */
    public String namespace() {
        return namespace;
    }

    /**
     * Waits until the Microcks service is ready.
     *
     * @param timeout The maximum duration to wait for the service to be ready.
     */
    public void waitUntilReady(final Duration timeout) {
        if (!ready) {
            await().atMost(timeout).until(() -> {
                Deployment deployment = kubernetesClient
                        .resources(Deployment.class)
                        .inNamespace(namespace())
                        .withName("microcks")
                        .get();
                return ready = deployment != null
                        && deployment.getStatus() != null
                        && deployment.getStatus().getAvailableReplicas() != null
                        && deployment.getStatus().getAvailableReplicas() >= 1;
            });
        }
    }

    /**
     * Uploads a list of URLs to the Microcks service. This method will upload each URL as a separate artifact, with the
     * first URL being treated as the main artifact.
     *
     * @param urls The list of URLs to upload.
     * @throws Exception If an error occurs during the upload.
     */
    public void upload(final List<URL> urls) throws Exception {
        try (LocalPortForward portForward = kubernetesClient
                .services()
                .inNamespace(namespace())
                .withName("microcks")
                .portForward(8080)) {
            URI address = new URI("http://localhost:" + portForward.getLocalPort() + "/api/");
            boolean isMainArtifact = true;
            for (URL url : urls) {
                String filename = Path.of(url.getPath()).getFileName().toString();
                try (InputStream inputStream = url.openStream()) {
                    upload(address, inputStream.readAllBytes(), filename, isMainArtifact);
                }
                isMainArtifact = false;
            }
        }
    }

    /**
     * Uploads a document to the Microcks service.
     *
     * @param address The URI of the Microcks service.
     * @param document The document content as a byte array.
     * @param name The name of the document.
     * @param isMainArtifact Whether this is the main artifact or not.
     * @throws Exception If an error occurs during the upload.
     */
    public void upload(final URI address, final byte[] document, final String name, final boolean isMainArtifact)
            throws Exception {
        String boundary = UUID.randomUUID().toString();

        ByteArrayOutputStream baos = new ByteArrayOutputStream();
        try (PrintStream writer = new PrintStream(baos, false, StandardCharsets.UTF_8)) {
            writer.printf("--%s\r\n", boundary);
            writer.printf("Content-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\n", name);
            writer.print("Content-Type: application/octet-stream\r\n\r\n");
            writer.write(document);
            writer.print("\r\n");
            writer.printf("--%s--\r\n", boundary);
        }

        Duration timeout = Duration.ofSeconds(30);
        HttpRequest request = HttpRequest.newBuilder()
                .uri(address.resolve("./artifact/upload?mainArtifact=" + isMainArtifact))
                .header("Content-Type", "multipart/form-data; boundary=" + boundary)
                .POST(HttpRequest.BodyPublishers.ofByteArray(baos.toByteArray()))
                .timeout(timeout)
                .build();

        try (HttpClient httpClient =
                HttpClient.newBuilder().connectTimeout(timeout).build()) {
            HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
            if (response.statusCode() != 200 && response.statusCode() != 201) {
                throw new IOException(
                        "Failed to upload " + name + ": " + response.statusCode() + " - " + response.body());
            }
        }
    }

    @Override
    public void close() throws Exception {
        helmClientContext.helmClient.uninstallChart("microcks");
        kubernetesClient.namespaces().withName(namespace()).delete();
        await().atMost(Duration.ofSeconds(30))
                .until(() -> kubernetesClient.namespaces().withName(namespace()).get() == null);
        helmClientContext.close();
    }
}
