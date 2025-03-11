// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.model.commonspec.OpenTelemetryCollector;
import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.ConsensusNode;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import io.fabric8.kubernetes.api.model.Affinity;
import io.fabric8.kubernetes.api.model.AffinityBuilder;
import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodResourceClaim;
import io.fabric8.kubernetes.api.model.PodResourceClaimBuilder;
import io.fabric8.kubernetes.api.model.PodSecurityContext;
import io.fabric8.kubernetes.api.model.PodSecurityContextBuilder;
import io.fabric8.kubernetes.api.model.PodTemplateSpec;
import io.fabric8.kubernetes.api.model.Quantity;
import io.fabric8.kubernetes.api.model.ResourceRequirements;
import io.fabric8.kubernetes.api.model.ResourceRequirementsBuilder;
import io.fabric8.kubernetes.api.model.Toleration;
import io.fabric8.kubernetes.api.model.TolerationBuilder;
import io.fabric8.kubernetes.api.model.TopologySpreadConstraint;
import io.fabric8.kubernetes.api.model.TopologySpreadConstraintBuilder;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

public class ConsensusCapsulePodPropertiesTest {
    BeanFactory factory = new BeanFactory().maxList(5).maxMap(5).nullProbability(0.2);
    CapsuleDeployment capsulePod = new CapsuleDeployment();

    @Test
    void testProperties() {
        factory.suppliers(new LinkedHashMap<>(Map.of(
                        Quantity.class::equals,
                        (f, t) -> new Quantity(f.nonNull(Integer.class).toString()))))
                .nonNulls(PodProperties.class)
                .limit(10)
                .forEach(this::testProperties);
    }

    @Test
    void testContainers() {
        factory.randoms(String.class).limit(10).forEach(this::testContainers);
    }

    void testProperties(final PodProperties src) {
        // copy pod properties to new pod builder and construct pod
        Deployment deployment =
                capsulePod.copyPodProperties(src, new DeploymentBuilder()).build();
        PodTemplateSpec pod = deployment.getSpec().getTemplate();
        // verify the pod properties are equivalent to the values in the pod spec
        assertThat(pod.getSpec().getAffinity())
                .isEqualTo(nullable(src.getAffinity(), Affinity::edit, AffinityBuilder::build));
        assertThat(pod.getSpec().getNodeSelector()).isEqualTo(src.getNodeSelector());
        assertThat(pod.getSpec().getPreemptionPolicy())
                .isEqualTo(nullable(src.getPreemptionPolicy(), PodProperties.PreemptionPolicy::getValue));
        assertThat(pod.getSpec().getPriority()).isEqualTo(src.getPriority());
        assertThat(pod.getSpec().getPriorityClassName()).isEqualTo(src.getPriorityClassName());
        assertThat(pod.getSpec().getResourceClaims())
                .isEqualTo(rebuild(
                        src.getResourceClaims(),
                        PodResourceClaim::new,
                        PodResourceClaim::edit,
                        PodResourceClaimBuilder::build));
        assertThat(pod.getSpec().getSecurityContext())
                .isEqualTo(
                        nullable(src.getSecurityContext(), PodSecurityContext::edit, PodSecurityContextBuilder::build));
        assertThat(pod.getSpec().getServiceAccountName()).isEqualTo(src.getServiceAccountName());
        assertThat(pod.getSpec().getTolerations())
                .isEqualTo(rebuild(src.getTolerations(), Toleration::new, Toleration::edit, TolerationBuilder::build));
        assertThat(pod.getSpec().getTerminationGracePeriodSeconds()).isEqualTo(src.getTerminationGracePeriodSeconds());
        assertThat(pod.getSpec().getTopologySpreadConstraints())
                .isEqualTo(rebuild(
                        src.getTopologySpreadConstraints(),
                        TopologySpreadConstraint::new,
                        TopologySpreadConstraint::edit,
                        TopologySpreadConstraintBuilder::build));
    }

    void testContainers(final String suffix) {
        // construct pod properties with container resources for each container
        PodProperties podProperties = edit(new PodProperties())
                .with(builder -> builder.editContainers()
                        .editBackupUploader()
                        .withName("backupUploader-" + suffix)
                        .withResources(factory.random(ResourceRequirements.class)))
                .with(builder -> builder.editContainers()
                        .editConsensusNode()
                        .withName("consensusNode-" + suffix)
                        .withResources(factory.random(ResourceRequirements.class)))
                .with(builder -> builder.editContainers()
                        .editEventsStreamUploader()
                        .withName("eventsStreamUploader-" + suffix)
                        .withResources(factory.random(ResourceRequirements.class)))
                .with(builder -> builder.editContainers()
                        .editRecordStreamSidecarFilesUploader()
                        .withName("recordStreamSidecarFilesUploader-" + suffix)
                        .withResources(factory.random(ResourceRequirements.class)))
                .with(builder -> builder.editContainers()
                        .editRecordStreamUploader()
                        .withName("recordStreamUploader-" + suffix)
                        .withResources(factory.random(ResourceRequirements.class)))
                .with(builder -> builder.editContainers()
                        .editOpenTelemetryCollector()
                        .withResources(factory.random(ResourceRequirements.class)))
                .build();
        // containers in pod properties
        Containers containers = podProperties.getContainers();
        // construct a pod which has empty containers with names matching the pod properties
        Pod pod = edit(new Pod())
                .with(builder -> builder.editSpec()
                        .addNewContainer()
                        .withName(containers.getBackupUploader().getName()))
                .with(builder -> builder.editSpec()
                        .addNewContainer()
                        .withName(containers.getConsensusNode().getName()))
                .with(builder -> builder.editSpec()
                        .addNewContainer()
                        .withName(containers.getEventsStreamUploader().getName()))
                .with(builder -> builder.editSpec()
                        .addNewContainer()
                        .withName(
                                containers.getRecordStreamSidecarFilesUploader().getName()))
                .with(builder -> builder.editSpec()
                        .addNewContainer()
                        .withName(containers.getRecordStreamUploader().getName()))
                .with(builder -> builder.editSpec()
                        .addNewContainer()
                        .withName(containers.getOpenTelemetryCollector().getName()))
                .build();
        // copy pod properties to pod builder and construct new pod
        Deployment deployment = edit(new Deployment())
                .with(builder -> builder.editSpec().editTemplate().withSpec(pod.getSpec()))
                .build();
        deployment =
                capsulePod.copyPodProperties(podProperties, deployment.edit()).build();
        // create a map from container name to containers in pod
        Map<String, Container> map = deployment.getSpec().getTemplate().getSpec().getContainers().stream()
                .collect(Collectors.toMap(Container::getName, Function.identity()));
        // verify the containers in pod properties are equivalent to corresponding container in pod
        assertThat(map.get(containers.getBackupUploader().getName()).getResources())
                .isEqualTo(rebuild(containers.getBackupUploader(), StreamUploader::getResources));
        assertThat(map.get(containers.getConsensusNode().getName()).getResources())
                .isEqualTo(rebuild(containers.getConsensusNode(), ConsensusNode::getResources));
        assertThat(map.get(containers.getEventsStreamUploader().getName()).getResources())
                .isEqualTo(rebuild(containers.getEventsStreamUploader(), StreamUploader::getResources));
        assertThat(map.get(containers.getRecordStreamSidecarFilesUploader().getName())
                        .getResources())
                .isEqualTo(rebuild(containers.getRecordStreamSidecarFilesUploader(), StreamUploader::getResources));
        assertThat(map.get(containers.getRecordStreamUploader().getName()).getResources())
                .isEqualTo(rebuild(containers.getRecordStreamUploader(), StreamUploader::getResources));
        assertThat(map.get(containers.getOpenTelemetryCollector().getName()).getResources())
                .isEqualTo(rebuild(containers.getOpenTelemetryCollector(), OpenTelemetryCollector::getResources));
    }

    <T, E> List<T> rebuild(
            final List<T> list, final Supplier<T> create, final Function<T, E> edit, final Function<E, T> build) {
        return list == null
                ? null
                : list.stream()
                        .map(x -> x == null ? create.get() : edit.andThen(build).apply(x))
                        .toList();
    }

    <T> ResourceRequirements rebuild(final T object, final Function<T, ResourceRequirements> function) {
        return nullable(object, function, ResourceRequirements::edit, ResourceRequirementsBuilder::build);
    }

    public static boolean isNull(final Object... objects) {
        for (Object object : objects) {
            if (object == null) {
                return true;
            }
        }
        return false;
    }

    public static <A, B> B nullable(final A a, final Function<A, B> ab) {
        return isNull(a, ab) ? null : ab.apply(a);
    }

    public static <A, B, C> C nullable(final A a, final Function<A, B> ab, final Function<B, C> bc) {
        return nullable(nullable(a, ab), bc);
    }

    public static <A, B, C, D> D nullable(
            final A a, final Function<A, B> ab, final Function<B, C> bc, final Function<C, D> cd) {
        return nullable(nullable(a, ab), bc, cd);
    }
}
