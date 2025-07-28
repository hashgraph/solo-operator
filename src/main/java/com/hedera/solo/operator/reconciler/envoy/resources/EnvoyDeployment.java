// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.envoy.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.data.AsciiArmor.base64Encode;
import static com.hedera.solo.operator.library.model.Labels.soloOperatorDomain;
import static com.hedera.solo.operator.library.render.QuteUtils.generate;
import static com.hedera.solo.operator.library.render.QuteUtils.template;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.configMapName;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.deploymentName;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.labels;

import com.hedera.solo.operator.library.data.Digest;
import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.KeyToPath;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.quarkus.qute.Template;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.SequencedMap;
import java.util.Set;
import java.util.TreeMap;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/** Represents a Kubernetes Deployment resource for EnvoyProxy. */
public class EnvoyDeployment extends CRUDKubernetesDependentResource<Deployment, EnvoyProxy> {
    Template template = template("envoy/envoy-deploy.yaml");

    @Override
    protected Deployment desired(final EnvoyProxy primary, final Context<EnvoyProxy> context) {
        return edit(generate(Deployment.class, template, t -> t.data("primary", primary)
                        .data("env", env(primary))))
                .with(metadata -> metadata.editMetadata()
                        .withName(deploymentName(primary))
                        .addToLabels(labels(primary)))
                .with(spec -> spec.editSpec() // .withReplicas(2)
                        .editSelector()
                        .withMatchLabels(labels(primary)))
                .with(templateMetadata -> templateMetadata
                        .editSpec()
                        .editTemplate()
                        .editMetadata()
                        .addToLabels(labels(primary))
                        .addToAnnotations(
                                soloOperatorDomain("digest"),
                                Digest.SHA_384.hexOf(primary.getSpec().getFiles().entrySet().stream()
                                        .sorted(Map.Entry.comparingByKey())
                                        .flatMap(entry -> Stream.of(
                                                entry.getKey().getBytes(),
                                                entry.getValue().getBytes())))))
                .with(podSpec -> podSpec.editSpec()
                        .editTemplate()
                        .editSpec()
                        .addAllToVolumes(volumes(primary))
                        .editMatchingVolume(v -> "solo-bin".equals(v.getName()))
                        .editConfigMap()
                        .withName(configMapName(primary)))
                .with(envoyContainer -> envoyContainer
                        .editSpec()
                        .editTemplate()
                        .editSpec()
                        .editMatchingContainer(c -> "envoy".equals(c.getName()))
                        .withEnv(primary.getSpec().getEnv())
                        .addAllToVolumeMounts(volumeMounts(primary)))
                .build();
    }

    @Override
    protected ResourceID targetSecondaryResourceID(final EnvoyProxy primary, final Context<EnvoyProxy> context) {
        return new ResourceID(deploymentName(primary), primary.getMetadata().getNamespace());
    }

    /**
     * Extracts the environment variable names from the EnvoyProxy spec.
     *
     * @param primary the EnvoyProxy resource
     * @return a set of environment variable names
     */
    Set<String> env(final EnvoyProxy primary) {
        return primary.getSpec().getEnv().stream().map(EnvVar::getName).collect(Collectors.toSet());
    }

    /**
     * Creates a collection of VolumeMount objects based on the files defined in the EnvoyProxy spec.
     *
     * @param primary the EnvoyProxy resource
     * @return a collection of VolumeMount objects
     */
    Collection<VolumeMount> volumeMounts(final EnvoyProxy primary) {
        List<VolumeMount> mounts = new ArrayList<>();
        for (String path : files(primary).keySet()) {
            mounts.add(edit(new VolumeMount())
                    .with(vm -> vm.withName(volumeName(mounts.size())).withMountPath(path))
                    .build());
        }
        return mounts;
    }

    /**
     * Creates a collection of Volume objects based on the files defined in the EnvoyProxy spec.
     *
     * @param primary the EnvoyProxy resource
     * @return a collection of Volume objects
     */
    Collection<Volume> volumes(final EnvoyProxy primary) {
        List<Volume> volumes = new ArrayList<>();
        for (Map.Entry<String, List<KeyToPath>> entry : files(primary).entrySet()) {
            volumes.add(edit(new Volume())
                    .with(v -> v.withName(volumeName(volumes.size()))
                            .editConfigMap()
                            .withName(configMapName(primary))
                            .withItems(entry.getValue()))
                    .build());
        }
        return volumes;
    }

    /**
     * Groups the files in the EnvoyProxy spec by their parent directory and maps them to a list of KeyToPath objects.
     *
     * @param primary the EnvoyProxy resource
     * @return a sequenced map where keys are parent directories and values are lists of KeyToPath objects
     */
    SequencedMap<String, List<KeyToPath>> files(final EnvoyProxy primary) {
        return primary.getSpec().getFiles().keySet().stream()
                .map(Path::of)
                .sorted()
                .collect(Collectors.groupingBy(
                        path -> path.getParent().toString(),
                        TreeMap::new,
                        Collectors.mapping(
                                path -> new KeyToPath(
                                        base64Encode(path.toString()),
                                        null,
                                        path.getFileName().toString()),
                                Collectors.toList())));
    }

    /**
     * Generates a unique volume name based on the counter.
     *
     * @param counter the index of the volume
     * @return a unique volume name
     */
    String volumeName(final int counter) {
        return "volume-" + counter;
    }
}
