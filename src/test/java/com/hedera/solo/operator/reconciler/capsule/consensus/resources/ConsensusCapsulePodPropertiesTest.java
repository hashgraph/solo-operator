// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.ConsensusNode;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.OpenTelemetryCollector;
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

/**
 * This class contains unit tests for the PodProperties of a ConsensusCapsule, including testing its properties and
 * containers.
 */
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

    /**
     * This method runs a series of tests to verify the properties of PodProperties.
     *
     * @param src The source PodProperties object to be tested.
     */
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

    /**
     * This method runs a series of tests to verify the containers in PodProperties.
     *
     * @param suffix A suffix string used to differentiate container names during testing.
     */
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

    /**
     * Rebuilds a list by applying optional editing and building functions based on whether each element is null or not.
     *
     * <p>If the list is null, it returns null. Otherwise, it applies the provided create function to generate new
     * elements when needed, followed by the edit function (if applicable), and finally the build function to convert
     * the result back to the original type.
     *
     * @param <T> The type of objects in the list.
     * @param <E> A intermediate type used for editing before building.
     * @param list The list to be rebuilt, potentially containing null elements.
     * @param create A supplier function to generate new elements when a null element is encountered.
     * @param edit An optional editing function that can transform existing elements.
     * @param build A function to convert the edited or original elements back into their original type.
     * @return A rebuilt list with potentially new elements created and modified as per the provided functions, or null
     *     if the input list was null.
     */
    <T, E> List<T> rebuild(
            final List<T> list, final Supplier<T> create, final Function<T, E> edit, final Function<E, T> build) {
        return list == null
                ? null
                : list.stream()
                        .map(x -> x == null ? create.get() : edit.andThen(build).apply(x))
                        .toList();
    }

    /**
     * Rebuilds a ResourceRequirements object based on the provided function, ensuring null safety by applying
     * conditional functions if needed.
     *
     * @param <T> The type of the object from which the ResourceRequirements are derived.
     * @param object The input object potentially containing ResourceRequirements data.
     * @param function A function that extracts and possibly edits a ResourceRequirements from the given object.
     * @return The rebuilt ResourceRequirements, or null if the input object was null or contained no valid
     *     ResourceRequirements.
     */
    <T> ResourceRequirements rebuild(final T object, final Function<T, ResourceRequirements> function) {
        return nullable(object, function, ResourceRequirements::edit, ResourceRequirementsBuilder::build);
    }

    /**
     * Checks if any of the provided objects are null.
     *
     * @param objects The objects to check for null values.
     * @return true if any object is null, false otherwise
     */
    public static boolean isNull(final Object... objects) {
        for (Object object : objects) {
            if (object == null) {
                return true;
            }
        }
        return false;
    }

    /**
     * Applies a function to an input only if it is not null, returning the result or null if the input is null.
     *
     * @param <A> The type of the input object.
     * @param <B> The type of the output object.
     * @param a The input object.
     * @param ab A function to apply if the input is not null.
     * @return The result of applying the function, or null if the input was null
     */
    public static <A, B> B nullable(final A a, final Function<A, B> ab) {
        return isNull(a, ab) ? null : ab.apply(a);
    }

    /**
     * Applies a series of functions to an input object in sequence, returning the result or null if any step results in
     * null.
     *
     * @param <A> The type of the input object.
     * @param <B> The intermediate type after applying the first function.
     * @param <C> The final type after applying all provided functions.
     * @param a The input object.
     * @param ab The first function to apply.
     * @param bc The second function to apply (optional).
     * @return The result of applying all functions, or null if any intermediate step results in null
     */
    public static <A, B, C> C nullable(final A a, final Function<A, B> ab, final Function<B, C> bc) {
        return nullable(nullable(a, ab), bc);
    }

    /**
     * Applies multiple functions in sequence to an input object, returning the result or null if any step results in
     * null.
     *
     * @param <A> The type of the input object.
     * @param <B> The intermediate type after applying the first function.
     * @param <C> The intermediate type after applying the second function.
     * @param <D> The final type after applying all provided functions.
     * @param a The input object.
     * @param ab The first function to apply.
     * @param bc The second function to apply (optional).
     * @param cd The third function to apply (optional).
     * @return The result of applying all functions, or null if any intermediate step results in null
     */
    public static <A, B, C, D> D nullable(
            final A a, final Function<A, B> ab, final Function<B, C> bc, final Function<C, D> cd) {
        return nullable(nullable(a, ab), bc, cd);
    }
}
