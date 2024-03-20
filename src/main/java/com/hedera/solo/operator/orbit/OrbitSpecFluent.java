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

public class OrbitSpecFluent<T extends OrbitSpecFluent<T>> extends BaseFluent<T> {

    private String lifetime;
    private List<String> finalizers;
    private Map<String, Object> additionalProperties;

    public OrbitSpecFluent() {}

    public OrbitSpecFluent(@Nullable final OrbitSpec spec) {
        this.copyInstance(spec);
    }

    protected void copyInstance(@Nullable final OrbitSpec instance) {
        if (instance != null) {
            withLifetime(instance.getLifetime());
            withFinalizers(instance.getFinalizers());
        }
    }

    @Nullable
    public String getLifetime() {
        return lifetime;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T withLifetime(@Nullable final String lifetime) {
        this.lifetime = lifetime;
        return (T) this;
    }

    public boolean hasLifetime() {
        return lifetime != null && !lifetime.isBlank();
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T addToFinalizers(final int index, @Nullable final String item) {
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.add(index, item);
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T setToFinalizers(final int index, @Nullable final String item) {
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.set(index, item);
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T addToFinalizers(@NotNull final String... items) {
        requireNonNull(items);
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.addAll(Arrays.asList(items));
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T addAllToFinalizers(@NotNull final Collection<String> items) {
        requireNonNull(items);
        finalizers = requireNonNullElseGet(finalizers, ArrayList::new);
        finalizers.addAll(items);
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T removeFromFinalizers(@NotNull final String... items) {
        requireNonNull(items);
        if (finalizers != null) {
            finalizers.removeAll(Arrays.asList(items));
        }
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T removeAllFromFinalizers(@NotNull final Collection<String> items) {
        requireNonNull(items);
        if (finalizers != null) {
            finalizers.removeAll(items);
        }
        return (T) this;
    }

    @Nullable
    public List<String> getFinalizers() {
        return finalizers;
    }

    @Nullable
    public String getFinalizer(final int index) {
        return finalizers.get(index);
    }

    @Nullable
    public String getFirstFinalizer() {
        return finalizers.getFirst();
    }

    @Nullable
    public String getLastFinalizer() {
        return finalizers.getLast();
    }

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

    public boolean hasFinalizers() {
        return finalizers != null && !finalizers.isEmpty();
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T addToAdditionalProperties(@NotNull final String key, @Nullable final Object value) {
        requireNonNull(key);
        additionalProperties = requireNonNullElseGet(additionalProperties, LinkedHashMap::new);
        additionalProperties.put(key, value);
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T addToAdditionalProperties(@Nullable final Map<String, Object> items) {
        if (items != null) {
            additionalProperties = requireNonNullElseGet(additionalProperties, LinkedHashMap::new);
            additionalProperties.putAll(items);
        }
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T removeFromAdditionalProperties(@NotNull final String key) {
        requireNonNull(key);
        if (additionalProperties != null) {
            additionalProperties.remove(key);
        }
        return (T) this;
    }

    @NotNull
    @SuppressWarnings("unchecked")
    public T removeAllFromAdditionalProperties(@NotNull final Map<String, Object> items) {
        requireNonNull(items);
        if (additionalProperties != null) {
            items.keySet().forEach(additionalProperties::remove);
        }
        return (T) this;
    }

    @Nullable
    public Map<String, Object> getAdditionalProperties() {
        return additionalProperties;
    }

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

    public boolean hasAdditionalProperties() {
        return additionalProperties != null && !additionalProperties.isEmpty();
    }

    @Override
    public boolean equals(final Object o) {
        if (this == o) return true;
        if (!(o instanceof OrbitSpecFluent<?> that)) return false;
        if (!super.equals(o)) return false;
        return Objects.equals(getLifetime(), that.getLifetime())
                && Objects.equals(getFinalizers(), that.getFinalizers())
                && Objects.equals(getAdditionalProperties(), that.getAdditionalProperties());
    }

    @Override
    public int hashCode() {
        return Objects.hash(super.hashCode(), getLifetime(), getFinalizers(), getAdditionalProperties());
    }
}
