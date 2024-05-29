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

package com.hedera.solo.operator.reconciler.common;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.api.model.ObjectMetaFluent;
import io.smallrye.common.constraint.NotNull;
import io.smallrye.common.constraint.Nullable;
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

    /** The prefix for all operator labels. */
    public static final String OPERATOR_PREFIX = "operator.solo.hedera.com/";

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
    public static String validName(String name, String suffix) {
        if (name.length() + suffix.length() > 63) {
            name = name.substring(0, 63 - suffix.length());
        }
        return name + suffix;
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
        final BiPredicate<String, String> actualFilter = Objects.requireNonNullElse(filter, DEFAULT_FILTER);

        for (final Map.Entry<String, String> entry : srcLabels.entrySet()) {
            if (actualFilter.test(entry.getKey(), entry.getValue())) {
                setter.accept(dst, entry.getKey(), entry.getValue());
            }
        }

        return dst;
    }
}
