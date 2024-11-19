// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.common;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_ACCOUNT_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_NAME;
import static com.hedera.solo.operator.reconciler.common.Constants.OPERATOR_PREFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.api.model.ObjectMetaFluent;
import io.smallrye.common.constraint.NotNull;
import io.smallrye.common.constraint.Nullable;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;
import java.util.function.BiPredicate;
import java.util.function.Function;
import org.apache.commons.lang3.function.TriConsumer;

/** Provides a set of constants and utility methods for Kubernetes Metadata. */
public final class Metadata {

    /** Private constructor to prevent instantiation of this utility class. */
    private Metadata() {
        throw new UnsupportedOperationException("Utility class");
    }

    /** The default filter that accepts all labels. */
    public static final BiPredicate<String, String> DEFAULT_FILTER = (key, value) -> true;

    /** The filter that accepts only operator specific labels. */
    public static final BiPredicate<String, String> OPERATOR_FILTER = (key, value) -> key.startsWith(OPERATOR_PREFIX);

    /** The label key for the Orbit name. */
    public static final String ORBIT_NAME = OPERATOR_PREFIX + "orbit-name";

    /** The label key for the Orbit namespace. */
    public static final String ORBIT_NAMESPACE = OPERATOR_PREFIX + "orbit-namespace";

    /**
     * Copies the selected metadata from the given Orbit object to the target object implementing {@link HasMetadata}.
     * See {@link #applyOrbitMetaTo(Orbit, ObjectMetaFluent)} for more details.
     *
     * @param orbit the Orbit object from which the metadata should be copied/
     * @param target the target object implementing {@link HasMetadata} to which the Orbit metadata should be applied/
     * @throws NullPointerException if any of the arguments is {@code null}.
     * @see #applyOrbitMetaTo(Orbit, ObjectMetaFluent)
     */
    public static void applyOrbitMetaTo(@NotNull final Orbit orbit, @NotNull final HasMetadata target) {
        Objects.requireNonNull(orbit, "orbit must not be null");
        Objects.requireNonNull(target, "target must not be null");
        target.setMetadata(applyOrbitMetaTo(orbit, target.getMetadata().edit()).build());
    }

    /**
     * Copies the selected metadata from the given Orbit object to the fluent builder implementing
     * {@link ObjectMetaFluent}.
     *
     * <p>The following metadata is copied:
     *
     * <ul>
     *   <li>Orbit name and namespace are added as labels.
     *   <li>Operator specific labels and annotations are copied.
     * </ul>
     *
     * @param orbit the Orbit object from which the metadata should be copied.
     * @param metadata the fluent builder to which the Orbit metadata should be applied.
     * @param <A> the type of the fluent builder.
     * @return the fluent builder with the Orbit metadata applied.
     * @throws NullPointerException if any of the arguments is {@code null}.
     */
    @NotNull
    public static <A extends ObjectMetaFluent<A>> A applyOrbitMetaTo(
            @NotNull final Orbit orbit, @NotNull final A metadata) {
        Objects.requireNonNull(orbit, "orbit must not be null");
        Objects.requireNonNull(metadata, "metadata must not be null");

        metadata.addToLabels(ORBIT_NAME, orbit.getMetadata().getName());
        metadata.addToLabels(ORBIT_NAMESPACE, orbit.getMetadata().getName());
        copyLabels(orbit, metadata, OPERATOR_FILTER);
        copyAnnotations(orbit, metadata, OPERATOR_FILTER);
        return metadata;
    }

    /**
     * Copies the annotations from the source object to the destination object. No filter is applied and all annotations
     * are copied.
     *
     * @param src the source object.
     * @param dst the destination object.
     * @throws NullPointerException if any of the arguments is {@code null}.
     */
    public static void copyAnnotations(@NotNull final HasMetadata src, @NotNull final HasMetadata dst) {
        copyAnnotations(src, dst, null);
    }

    /**
     * Copies the annotations from the source object to the destination fluent builder. No filter is applied and all
     * annotations are copied.
     *
     * @param src the source object.
     * @param dst the destination object.
     * @return the destination fluent builder with the annotations copied.
     * @throws NullPointerException if the source object is {@code null}.
     */
    public static <A extends ObjectMetaFluent<A>> A copyAnnotations(
            @NotNull final HasMetadata src, @NotNull final A dst) {
        return copyAnnotations(src, dst, null);
    }

    /**
     * Copies the annotations from the source object to the destination object. The filter is applied to the annotations
     * and only those that pass the filter are copied. If the filter is {@code null}, all annotations are copied.
     *
     * @param src the source object.
     * @param dst the destination object.
     * @param filter the filter to apply to the annotations.
     * @throws NullPointerException if the source or destination object is {@code null}.
     */
    public static void copyAnnotations(
            @NotNull final HasMetadata src,
            @NotNull final HasMetadata dst,
            @Nullable final BiPredicate<String, String> filter) {
        copyMap(src, dst, ObjectMeta::getAnnotations, ObjectMetaBuilder::addToAnnotations, filter);
    }

    /**
     * Copies the annotations from the source object to the destination fluent builder. The filter is applied to the
     * annotations and only those that pass the filter are copied. If the filter is {@code null}, all annotations are
     * copied.
     *
     * @param src the source object.
     * @param dst the destination object.
     * @param filter the filter to apply to the annotations.
     * @return the destination fluent builder with the annotations copied.
     * @throws NullPointerException if the source object or destination fluent builder is {@code null}.
     */
    public static <A extends ObjectMetaFluent<A>> A copyAnnotations(
            @NotNull final HasMetadata src, @NotNull final A dst, @Nullable final BiPredicate<String, String> filter) {
        return copyMap(src, dst, ObjectMeta::getAnnotations, ObjectMetaFluent::addToAnnotations, filter);
    }

    /**
     * Copies the labels from the source object to the destination object. No filter is applied and all labels are
     * copied.
     *
     * @param src the source object.
     * @param dst the destination object.
     * @throws NullPointerException if any of the arguments is {@code null}.
     */
    public static void copyLabels(@NotNull final HasMetadata src, @NotNull final HasMetadata dst) {
        copyLabels(src, dst, null);
    }

    /**
     * Copies the labels from the source object to the destination fluent builder. No filter is applied and all labels
     * are copied.
     *
     * @param src the source object.
     * @param dst the destination fluent builder.
     * @return the destination fluent builder with the labels copied.
     * @throws NullPointerException if the source object or destination fluent builder is {@code null}.
     */
    @NotNull
    public static <A extends ObjectMetaFluent<A>> A copyLabels(@NotNull final HasMetadata src, @NotNull final A dst) {
        return copyLabels(src, dst, null);
    }

    /**
     * Copies the labels from the source object to the destination object. The filter is applied to the labels and only
     * those that pass the filter are copied. If the filter is {@code null}, all labels are copied.
     *
     * @param src the source object.
     * @param dst the destination object.
     * @param filter the filter to apply to the labels.
     * @throws NullPointerException if the source or destination object is {@code null}.
     */
    public static void copyLabels(
            @NotNull final HasMetadata src,
            @NotNull final HasMetadata dst,
            @Nullable final BiPredicate<String, String> filter) {
        copyMap(src, dst, ObjectMeta::getLabels, ObjectMetaBuilder::addToLabels, filter);
    }

    /**
     * Copies the labels from the source object to the destination fluent builder. The filter is applied to the labels
     * and only those that pass the filter are copied. If the filter is {@code null}, all labels are copied.
     *
     * @param src the source object.
     * @param dst the destination fluent builder.
     * @param filter the filter to apply to the labels.
     * @return the destination fluent builder with the labels copied.
     * @throws NullPointerException if the source object or destination fluent builder is {@code null}.
     */
    @NotNull
    public static <A extends ObjectMetaFluent<A>> A copyLabels(
            @NotNull final HasMetadata src, @NotNull final A dst, @Nullable final BiPredicate<String, String> filter) {
        return copyMap(src, dst, ObjectMeta::getLabels, ObjectMetaFluent::addToLabels, filter);
    }

    /**
     * Shorten the name as necessary such that the shortened name and suffix do not exceed 63 characters.
     *
     * @param name the base name which may be shortened.
     * @param suffix the suffix which is presumed not to exceed 63 characters.
     * @return A string containing at most 63 characters
     * @see <a href="https://kubernetes.io/docs/concepts/overview/working-with-objects/names/">Object Names and IDs</a>
     */
    public static String validName(final String name, final String suffix) {
        String finalName = name;
        if (name.length() + suffix.length() > 63) {
            finalName = name.substring(0, 63 - suffix.length());
        }
        return finalName + suffix;
    }

    private static void copyMap(
            @NotNull final HasMetadata src,
            @NotNull final HasMetadata dst,
            @NotNull final Function<ObjectMeta, Map<String, String>> getter,
            @NotNull final TriConsumer<ObjectMetaBuilder, String, String> setter,
            @Nullable final BiPredicate<String, String> filter) {
        Objects.requireNonNull(src, "src must not be null");

        final ObjectMetaBuilder metadata = dst.getMetadata().edit();
        copyMap(src, metadata, getter, setter, filter);
        dst.setMetadata(metadata.build());
    }

    @NotNull
    private static <A extends ObjectMetaFluent<A>> A copyMap(
            @NotNull final HasMetadata src,
            @NotNull final A dst,
            @NotNull final Function<ObjectMeta, Map<String, String>> getter,
            @NotNull final TriConsumer<A, String, String> setter,
            @Nullable final BiPredicate<String, String> filter) {
        Objects.requireNonNull(src, "src must not be null");
        Objects.requireNonNull(dst, "dst must not be null");
        Objects.requireNonNull(getter, "getter must not be null");
        Objects.requireNonNull(setter, "setter must not be null");

        final Map<String, String> srcLabels = Objects.requireNonNullElseGet(getter.apply(src.getMetadata()), Map::of);
        return setLabels(dst, srcLabels, setter, filter);
    }

    private static <A extends ObjectMetaFluent<A>> A setLabels(
            @NotNull A dst,
            @NotNull Map<String, String> labels,
            @NotNull final TriConsumer<A, String, String> setter,
            @Nullable final BiPredicate<String, String> filter) {
        final BiPredicate<String, String> actualFilter = Objects.requireNonNullElse(filter, DEFAULT_FILTER);
        for (final Map.Entry<String, String> entry : labels.entrySet()) {
            if (actualFilter.test(entry.getKey(), entry.getValue())) {
                setter.accept(dst, entry.getKey(), entry.getValue());
            }
        }
        return dst;
    }

    public static String prepareNodeName(ConsensusCapsule cc) {
        // we are deriving the node name from node-id instead of using the name of the ConsensusCapsule
        return prepareNodeName(cc.getSpec().getNodeId());
    }

    public static String prepareNodeName(Long nodeId) {
        return Constants.NODE_NAME_FORMAT.formatted(nodeId);
    }

    /**
     * Apply standard labels to the given metadata.
     *
     * @param dst the metadata to which the labels should be applied.
     * @param name the instance name.
     * @param orbit the namespace.
     * @param parentComponent the parent component name.
     * @param componentType the component name.
     * @param customLabels the labels to apply.
     * @param <A> the type of the metadata fluent builder.
     * @return the metadata with the labels applied.
     */
    @NotNull
    public static <A extends ObjectMetaFluent<A>> A applyStandardLabels(
            @NotNull A dst,
            @NotNull String name,
            @NotNull String orbit,
            @NotNull String parentComponent,
            @NotNull String componentType,
            @Nullable Map<String, String> customLabels) {
        Objects.requireNonNull(dst, "dst must not be null");
        Objects.requireNonNull(name, "instanceName must not be null");
        Objects.requireNonNull(orbit, "orbit must not be null");
        Objects.requireNonNull(parentComponent, "parentComponent must not be null");
        Objects.requireNonNull(componentType, "componentType must not be null");

        Map<String, String> standardLabels =
                prepareStandardLabels(name, orbit, parentComponent, componentType, customLabels);
        return setLabels(dst, standardLabels, ObjectMetaFluent::addToLabels, DEFAULT_FILTER);
    }

    /**
     * Prepare standard labels for the given component.
     *
     * @param customLabels the labels to which the standard labels should be added.
     * @param name the name.
     * @param orbit the namespace.
     * @param parentComponent the parent component name.
     * @param componentType the component name.
     * @return the labels with the standard labels added.
     */
    public static Map<String, String> prepareStandardLabels(
            String name, String orbit, String parentComponent, String componentType, Map<String, String> customLabels) {
        Map<String, String> standardLabels = new HashMap<>();
        if (customLabels != null) {
            standardLabels.putAll(customLabels); // add first so that standard labels are overwritten if those exist
        }

        // solo-orbit specific labels
        standardLabels.put(Constants.LABEL_ORBIT, orbit);

        // k8s standard labels
        standardLabels.put(Constants.LABEL_K8S_NAME, name);
        standardLabels.put(Constants.LABEL_K8S_INSTANCE, name); // FIXME is this correct?
        standardLabels.put(Constants.LABEL_K8S_VERSION, Constants.SOLO_OPERATOR_VERSION);
        standardLabels.put(Constants.LABEL_K8S_COMPONENT, componentType);
        standardLabels.put(Constants.LABEL_K8S_PART_OF, parentComponent);
        standardLabels.put(Constants.LABEL_K8S_MANAGED_BY, Constants.SOLO_OPERATOR);

        return standardLabels;
    }

    public static Map<String, String> prepareConsensusNodeLabels(Long nodeId, String accountId) {
        final Map<String, String> labels = new HashMap<>(); // return a mutable hashmap
        labels.put(Constants.LABEL_APP, Constants.CONSENSUS_NODE);
        labels.put(LABEL_NODE_NAME, prepareNodeName(nodeId));
        labels.put(LABEL_ACCOUNT_ID, accountId);
        labels.put(LABEL_NODE_ID, nodeId.toString());
        return labels;
    }
}
