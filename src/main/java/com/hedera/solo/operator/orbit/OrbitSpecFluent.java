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

package com.hedera.solo.operator.orbit;

import static java.util.Objects.requireNonNull;
import static java.util.Objects.requireNonNullElseGet;

import io.fabric8.kubernetes.api.builder.BaseFluent;
import io.smallrye.common.constraint.NotNull;
import io.smallrye.common.constraint.Nullable;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.function.Predicate;

/**
 * The fluent API for the {@link OrbitSpecBuilder} implementation.
 *
 * @param <T> the type of the fluent builder implementation.
 */
public class OrbitSpecFluent<T extends OrbitSpecFluent<T>> extends BaseFluent<T> {

    /** Temporary storage for the lifetime of the {@code Orbit}. */
    private String lifetime;

    /** Temporary storage for the finalizers which should be attached to all resources in this {@code Orbit}. */
    private List<String> finalizers;

    /** Temporary storage for additional properties which are not explicitly part of the schema. */
    private Map<String, Object> additionalProperties;

    /** Creates a new {@code OrbitSpecFluent} with the default values. */
    public OrbitSpecFluent() {}

    /**
     * Creates new {@code OrbitSpecFluent} using the provided instance. This constructor copies the instance properties.
     *
     * @param instance the instance to copy.
     */
    public OrbitSpecFluent(@Nullable final OrbitSpec instance) {
        this.copyInstance(instance);
    }

    /**
     * Copies the properties of the provided instance to this fluent builder.
     *
     * @param instance the instance to copy. may be {@code null}.
     */
    protected void copyInstance(@Nullable final OrbitSpec instance) {
        if (instance != null) {
            withLifetime(instance.getLifetime());
            withFinalizers(instance.getFinalizers());
        }
    }

    /**
     * Standard accessor for the lifetime of the {@code Orbit}.
     *
     * @return the lifetime of the {@code Orbit}. may be a {@code null} reference.
     */
    @Nullable
    public String getLifetime() {
        return lifetime;
    }

    /**
     * Fluent setter for the lifetime of the {@code Orbit}.
     *
     * @param lifetime the lifetime of the {@code Orbit}. may be a {@code null} reference.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T withLifetime(@Nullable final String lifetime) {
        this.lifetime = lifetime;
        return (T) this;
    }

    /**
     * Checks for the existence of a non-{@code null} and non-blank lifetime.
     *
     * @return {@code true} if the lifetime is non-{@code null} and non-blank, {@code false} otherwise.
     */
    public boolean hasLifetime() {
        return lifetime != null && !lifetime.isBlank();
    }

    /**
     * Fluent setter for the finalizer at the specified index. This variant adds a new finalizer at the specified index.
     *
     * @param index the index at which to add the finalizer.
     * @param item the finalizer to be added.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T addToFinalizers(final int index, @Nullable final String item) {
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.add(index, item);
        return (T) this;
    }

    /**
     * Fluent setter for the finalizer at the specified index. This variant sets the finalizer at the specified index.
     *
     * @param index the index at which to set the finalizer.
     * @param item the finalizer to be set.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T setToFinalizers(final int index, @Nullable final String item) {
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.set(index, item);
        return (T) this;
    }

    /**
     * Fluent setter for a list of finalizers. This variant adds the specified finalizers.
     *
     * @param items the finalizers to be added.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T addToFinalizers(@NotNull final String... items) {
        requireNonNull(items);
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.addAll(Arrays.asList(items));
        return (T) this;
    }

    /**
     * Fluent setter for a collection of finalizers. This variant adds the specified finalizers.
     *
     * @param items the finalizers to be added.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T addAllToFinalizers(@NotNull final Collection<String> items) {
        requireNonNull(items);
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.addAll(items);
        return (T) this;
    }

    /**
     * Fluent setter which removes the specified finalizers.
     *
     * @param items the finalizers to be removed.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T removeFromFinalizers(@NotNull final String... items) {
        requireNonNull(items);
        if (finalizers != null) {
            finalizers.removeAll(Arrays.asList(items));
        }
        return (T) this;
    }

    /**
     * Fluent setter which removes the specified finalizers.
     *
     * @param items the finalizers to be removed.
     * @return
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T removeAllFromFinalizers(@NotNull final Collection<String> items) {
        requireNonNull(items);
        if (finalizers != null) {
            finalizers.removeAll(items);
        }
        return (T) this;
    }

    /**
     * Standard accessor for the finalizers which should be attached to all resources in this {@code Orbit}. This method
     * may return a {@code null} reference.
     *
     * @return a list of finalizers. may be a {@code null} reference.
     */
    @Nullable
    public List<String> getFinalizers() {
        return finalizers;
    }

    /**
     * Gets the finalizer at the specified index.
     *
     * @param index the index of the finalizer to retrieve.
     * @return the finalizer at the specified index. may be a {@code null} reference.
     * @throws IndexOutOfBoundsException if the index is out of range.
     * @throws NullPointerException if the finalizers list is a {@code null} reference.
     */
    @Nullable
    public String getFinalizer(final int index) {
        return finalizers.get(index);
    }

    /**
     * Gets the first finalizer in the list.
     *
     * @return the first finalizer in the list. may be a {@code null} reference.
     * @throws java.util.NoSuchElementException if the finalizers list is empty.
     * @throws NullPointerException if the finalizers list is a {@code null} reference.
     */
    @Nullable
    public String getFirstFinalizer() {
        return finalizers.getFirst();
    }

    /**
     * Gets the last finalizer in the list.
     *
     * @return the last finalizer in the list. may be a {@code null} reference.
     * @throws java.util.NoSuchElementException if the finalizers list is empty.
     * @throws NullPointerException if the finalizers list is a {@code null} reference.
     */
    @Nullable
    public String getLastFinalizer() {
        return finalizers.getLast();
    }

    /**
     * Gets the finalizer that matches the specified predicate.
     *
     * @param predicate the predicate used to match the finalizer.
     * @return the finalizer that matches the specified predicate. may be a {@code null} reference.
     * @throws NullPointerException if the predicate is a {@code null} reference.
     */
    @Nullable
    public String getMatchingFinalizer(@NotNull final Predicate<String> predicate) {
        requireNonNull(predicate);
        if (finalizers != null) {
            for (String finalizer : finalizers) {
                if (predicate.test(finalizer)) {
                    return finalizer;
                }
            }
        }
        return null;
    }

    /**
     * Checks for the existence of a finalizer that matches the specified predicate.
     *
     * @param predicate the predicate used to match the finalizer.
     * @return {@code true} if a matching finalizer exists, {@code false} otherwise.
     * @throws NullPointerException if the predicate is a {@code null} reference.
     */
    public boolean hasMatchingFinalizer(@NotNull final Predicate<String> predicate) {
        requireNonNull(predicate);
        if (finalizers != null) {
            for (String finalizer : finalizers) {
                if (predicate.test(finalizer)) {
                    return true;
                }
            }
        }
        return false;
    }

    /**
     * Fluent setter for the finalizers which should be attached to all resources in this {@code Orbit}. This method
     * replaces the existing finalizers with the specified list. Passing a {@code null} reference will clear the
     * finalizers.
     *
     * @param items the finalizers to be set. may be a {@code null} reference.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T withFinalizers(@Nullable final Collection<String> items) {
        if (items != null) {
            finalizers = new ArrayList<>(items);
        } else {
            finalizers = null;
        }
        return (T) this;
    }

    /**
     * Fluent setter for the finalizers which should be attached to all resources in this {@code Orbit}. This method
     * adds the specified list of finalizers.
     *
     * @param items the finalizers to be set. may be a {@code null} reference.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T withFinalizers(@Nullable final String... items) {
        if (finalizers != null) {
            finalizers.clear();
            _visitables.remove("finalizers");
        }

        if (items != null) {
            addToFinalizers(items);
        }

        return (T) this;
    }

    /**
     * Checks for the existence of at least one finalizer.
     *
     * @return {@code true} if there is at least one finalizer, {@code false} otherwise.
     */
    public boolean hasFinalizers() {
        return finalizers != null && !finalizers.isEmpty();
    }

    /**
     * Fluent setter which adds the specified additional property key/value pair.
     *
     * @param key the name of the additional property to be added. must not be a {@code null} reference.
     * @param value the value of the additional property to be added.
     * @return this fluent builder.
     * @throws NullPointerException if the given key argument is a {@code null} reference.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T addToAdditionalProperties(@NotNull final String key, @Nullable final Object value) {
        requireNonNull(key);
        additionalProperties = requireNonNullElseGet(additionalProperties, LinkedHashMap::new);
        additionalProperties.put(key, value);
        return (T) this;
    }

    /**
     * Fluent setter which adds the specified additional properties.
     *
     * @param items the additional properties to be added.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T addToAdditionalProperties(@Nullable final Map<String, Object> items) {
        if (items != null) {
            additionalProperties = requireNonNullElseGet(additionalProperties, LinkedHashMap::new);
            additionalProperties.putAll(items);
        }
        return (T) this;
    }

    /**
     * Fluent setter for which removes the specified additional property.
     *
     * @param key the name of the additional property to be removed.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T removeFromAdditionalProperties(@NotNull final String key) {
        requireNonNull(key);
        if (additionalProperties != null) {
            additionalProperties.remove(key);
        }
        return (T) this;
    }

    /**
     * Fluent setter which removes the specified additional properties.
     *
     * @param items the additional properties to be removed.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T removeAllFromAdditionalProperties(@NotNull final Map<String, Object> items) {
        requireNonNull(items);
        if (additionalProperties != null) {
            items.keySet().forEach(additionalProperties::remove);
        }
        return (T) this;
    }

    /**
     * Standard accessor for the additional properties which are not explicitly defined in the schema. This method may
     * return a {@code null} reference.
     *
     * @return a map of additional properties. may be a {@code null} reference.
     */
    @Nullable
    public Map<String, Object> getAdditionalProperties() {
        return additionalProperties;
    }

    /**
     * Fluent setter for the additional properties which are not explicitly defined in the schema. This method replaces
     * the entire map of additional properties with the given map. Passing a {@code null} reference will effectively
     * clear the map of additional properties.
     *
     * @param items a map of additional properties. may be a {@code null} reference.
     * @return this fluent builder.
     */
    @NotNull
    @SuppressWarnings("unchecked")
    public T withAdditionalProperties(@Nullable final Map<String, Object> items) {
        if (items != null) {
            additionalProperties = new LinkedHashMap<>(items);
        } else {
            additionalProperties = null;
        }
        return (T) this;
    }

    /**
     * Checks for the existence of additional properties.
     *
     * @return {@code true} if there are additional properties, {@code false} otherwise.
     */
    public boolean hasAdditionalProperties() {
        return additionalProperties != null && !additionalProperties.isEmpty();
    }

    /** {@inheritDoc} */
    @Override
    public boolean equals(final Object o) {
        if (this == o) return true;
        if (!(o instanceof OrbitSpecFluent<?> that)) return false;
        if (!super.equals(o)) return false;
        return Objects.equals(getLifetime(), that.getLifetime())
                && Objects.equals(getFinalizers(), that.getFinalizers())
                && Objects.equals(getAdditionalProperties(), that.getAdditionalProperties());
    }

    /** {@inheritDoc} */
    @Override
    public int hashCode() {
        return Objects.hash(super.hashCode(), getLifetime(), getFinalizers(), getAdditionalProperties());
    }
}
