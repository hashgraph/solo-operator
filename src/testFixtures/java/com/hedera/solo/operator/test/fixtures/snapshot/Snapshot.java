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

package com.hedera.solo.operator.test.fixtures.snapshot;

import static com.hedera.solo.operator.test.fixtures.snapshot.CallStack.calledFrom;

import io.fabric8.kubernetes.api.model.APIResource;
import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.GenericKubernetesResource;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.apiextensions.v1.CustomResourceDefinition;
import io.fabric8.kubernetes.api.model.apiextensions.v1.CustomResourceDefinitionVersion;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import io.fabric8.kubernetes.client.dsl.ContainerResource;
import io.fabric8.kubernetes.client.dsl.ExecWatch;
import io.fabric8.kubernetes.client.dsl.PodResource;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.lang.reflect.Method;
import java.net.URL;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import java.util.jar.JarFile;
import java.util.zip.ZipEntry;

public class Snapshot {
    public static final String NO_NAMESPACE = null;
    private static final String RESOURCE_LOCATION = "META-INF/fabric8";
    private static final String JAR_FILE = "jar:file:";
    private static final String FILE = "file:";
    private final KubernetesSerialization serialization = new KubernetesSerialization();
    private final SnapshotWriter writer;
    private final KubernetesClient client;
    private final List<String> namespaces;
    private final boolean noNamespace;

    public Snapshot(final SnapshotWriter writer, final KubernetesClient client, final String... namespaces) {
        this.writer = writer;
        this.client = client;
        this.namespaces =
                Arrays.stream(namespaces).filter(Objects::nonNull).distinct().toList();
        noNamespace = Arrays.stream(namespaces).anyMatch(Objects::isNull);
    }

    public static Snapshot snapshot(final KubernetesClient client, final String... namespaces) throws IOException {
        return snapshot(new DefaultSnapshotWriter(calledFrom()), client, namespaces);
    }

    public static Snapshot snapshot(final Method method, final KubernetesClient client, final String... namespaces)
            throws IOException {
        return snapshot(new DefaultSnapshotWriter(method), client, namespaces);
    }

    public static Snapshot snapshot(
            final SnapshotWriter writer, final KubernetesClient client, final String... namespaces) throws IOException {
        return new Snapshot(writer, client, namespaces).resources().logs();
    }

    public Snapshot resources() throws IOException {
        return resources(groupVersions());
    }

    public Snapshot resources(final String... groupVersions) throws IOException {
        for (String groupVersion : groupVersions) {
            for (APIResource resource : client.getApiResources(groupVersion).getResources()) {
                if (resource.getVerbs().contains("list")) {
                    if (resource.getNamespaced()) {
                        for (String namespace : namespaces) {
                            for (GenericKubernetesResource object : client.genericKubernetesResources(
                                            groupVersion, resource.getKind())
                                    .inNamespace(namespace)
                                    .list()
                                    .getItems()) {
                                writer.write(
                                        object,
                                        "namespaced",
                                        namespace,
                                        resource.getName(),
                                        object.getMetadata().getName());
                            }
                        }
                    } else if (noNamespace) {
                        for (GenericKubernetesResource object : client.genericKubernetesResources(
                                        groupVersion, resource.getKind())
                                .list()
                                .getItems()) {
                            writer.write(
                                    object,
                                    "no-namespace",
                                    resource.getName(),
                                    object.getMetadata().getName());
                        }
                    }
                }
            }
        }
        return this;
    }

    public Snapshot logs() throws IOException {
        for (String namespace : namespaces) {
            for (Pod pod : client.pods().inNamespace(namespace).list().getItems()) {
                for (Container container : pod.getSpec().getContainers()) {
                    ContainerResource resource = containerResource(pod, container);
                    LogResult result = new LogResult(resource.usingTimestamps().getLog());
                    try {
                        result.previous =
                                resource.usingTimestamps().terminated().getLog();
                    } catch (KubernetesClientException e) {
                    }
                    writer.write(result, "logs", namespace, pod.getMetadata().getName(), container.getName());
                }
            }
        }
        return this;
    }

    public Snapshot exec(final String name, final String selector, final String container, final String script)
            throws IOException {
        return exec(name, selector, container, "sh", "-c", script);
    }

    public Snapshot exec(final String name, final String selector, final String container, final String... exec)
            throws IOException {
        for (String namespace : namespaces) {
            for (Pod pod : client.pods()
                    .inNamespace(namespace)
                    .withLabelSelector(selector)
                    .list()
                    .getItems()) {
                PodResource resource = podResource(pod);
                ByteArrayOutputStream out = new ByteArrayOutputStream();
                ByteArrayOutputStream err = new ByteArrayOutputStream();
                try (ExecWatch watch = resource.inContainer(container)
                        .writingOutput(out)
                        .writingError(err)
                        .exec(exec)) {
                    ExecResult result = new ExecResult(exec, watch.exitCode().join(), out.toString(), err.toString());
                    writer.write(
                            result, "exec", name, namespace, pod.getMetadata().getName(), container);
                }
            }
        }
        return this;
    }

    private PodResource podResource(final Pod pod) {
        return client.pods()
                .inNamespace(pod.getMetadata().getNamespace())
                .withName(pod.getMetadata().getName());
    }

    private ContainerResource containerResource(final Pod pod, final Container container) {
        return podResource(pod).inContainer(container.getName());
    }

    private List<String> listResources() throws IOException {
        List<String> result = new ArrayList<>();
        for (URL url : Collections.list(ClassLoader.getSystemResources(RESOURCE_LOCATION))) {
            String urlString = url.toExternalForm();
            if (urlString.startsWith(JAR_FILE)) {
                String jarFileName =
                        urlString.substring(JAR_FILE.length(), urlString.length() - RESOURCE_LOCATION.length() - 2);
                try (JarFile jarFile = new JarFile(jarFileName)) {
                    Collections.list(jarFile.entries()).stream()
                            .filter(entry -> entry.getName().startsWith(RESOURCE_LOCATION))
                            .filter(entry -> !entry.isDirectory())
                            .map(ZipEntry::getName)
                            .forEach(result::add);
                }
            }
            if (urlString.startsWith(FILE)) {
                for (String entry : Objects.requireNonNull(new File(url.getFile()).list())) {
                    result.add(String.join("/", RESOURCE_LOCATION, entry));
                }
            }
        }
        return result;
    }

    private String[] groupVersions() throws IOException {
        Set<String> result = new LinkedHashSet<>(List.of("v1"));
        for (String resource : listResources()) {
            for (URL url : Collections.list(ClassLoader.getSystemResources(resource))) {
                try (InputStream inputStream = url.openStream()) {
                    Object object = serialization.unmarshal(inputStream);
                    if (object instanceof CustomResourceDefinition crd) {
                        for (CustomResourceDefinitionVersion version :
                                crd.getSpec().getVersions()) {
                            result.add(String.join("/", crd.getSpec().getGroup(), version.getName()));
                        }
                    }
                }
            }
        }
        return result.toArray(String[]::new);
    }
}
