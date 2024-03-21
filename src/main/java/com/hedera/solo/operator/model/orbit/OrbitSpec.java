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

package com.hedera.solo.operator.model.orbit;

import static java.util.Objects.requireNonNull;
import static java.util.Objects.requireNonNullElseGet;

import com.fasterxml.jackson.annotation.JsonAnyGetter;
import com.fasterxml.jackson.annotation.JsonAnySetter;
import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.smallrye.common.constraint.NotNull;
import io.smallrye.common.constraint.Nullable;
import java.util.LinkedHashMap;
import java.util.LinkedList;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/** The desired state of an {@code Orbit}. */
@JsonPropertyOrder({"apiVersion", "kind", "metadata", "lifetime", "finalizers"})
public class OrbitSpec implements Editable<OrbitSpecBuilder>, KubernetesResource {

    /**
     * The lifetime of the {@code Orbit}. The string value must be one of the following:
     *
     * <ul>
     *   <li>A standard textual Java duration (eg: {@code P2DT15H}).
     *   <li>A human readable duration (eg: {@code 2 days 15 hours}).
     * </ul>
     */
    @JsonProperty("lifetime")
    @JsonInclude(JsonInclude.Include.NON_NULL)
    private String lifetime;

    /** A list of finalizers which should be attached to all resources in this {@code Orbit}. */
    @JsonProperty("finalizers")
    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private List<String> finalizers;

    /** Additional properties which are not explicitly defined in the schema. */
    @JsonIgnore
    private final Map<String, Object> additionalProperties = new LinkedHashMap<>();

    /** Constructs a new {@code OrbitSpec} with default values. */
    public OrbitSpec() {
        this.finalizers = new LinkedList<>();
    }

    /**
     * Constructs a new {@code OrbitSpec} with the given lifetime and finalizers.
     *
     * @param lifetime the lifetime of the {@code Orbit}. may be a {@code null} reference.
     * @param finalizers a list of finalizers which should be attached to all resources in this {@code Orbit}. may be a
     *     {@code null} reference.
     */
    public OrbitSpec(@Nullable final String lifetime, @Nullable final List<String> finalizers) {
        this.lifetime = lifetime;
        this.finalizers = new LinkedList<>(requireNonNullElseGet(finalizers, LinkedList::new));
    }

    /**
     * Fluent accessor for the lifetime of the {@code Orbit}. May return a {@code null} reference.
     *
     * @return the lifetime of the {@code Orbit}; or {@code null} if no value is present.
     * @see #getLifetime()
     */
    @JsonIgnore
    @Nullable
    public String lifetime() {
        return lifetime;
    }

    /**
     * Gets the lifetime of the {@code Orbit}. May return a {@code null} reference.
     *
     * <p>The value, if present, must be one of the following:
     *
     * <ul>
     *   <li>A standard textual Java duration (eg: {@code P2DT15H}).
     *   <li>A human readable duration (eg: {@code 2 days 15 hours}).
     * </ul>
     *
     * @return the lifetime of the {@code Orbit}; or {@code null} if no value is present.
     * @see #lifetime()
     */
    @JsonProperty("lifetime")
    @Nullable
    public String getLifetime() {
        return lifetime;
    }

    /**
     * Sets the lifetime of the {@code Orbit}. Passing a {@code null} reference will effectively clear the lifetime of
     * the {@code Orbit}.
     *
     * <p>The value, if provided, must be one of the following:
     *
     * <ul>
     *   <li>A standard textual Java duration (eg: {@code P2DT15H}).
     *   <li>A human readable duration (eg: {@code 2 days 15 hours}).
     * </ul>
     *
     * @param lifetime the lifetime of the {@code Orbit}. may be a {@code null} reference.
     */
    @JsonProperty("lifetime")
    public void setLifetime(@Nullable final String lifetime) {
        this.lifetime = lifetime;
    }

    /**
     * Fluent accessor for the list of finalizers which should be attached to all resources in this {@code Orbit}.
     *
     * @return a list of finalizers. guaranteed to be non-{@code null}.
     * @see #getFinalizers()
     */
    @JsonIgnore
    @NotNull
    public List<String> finalizers() {
        return finalizers;
    }

    /**
     * Gets the list of finalizers which should be attached to all resources in this {@code Orbit}.
     *
     * @return a list of finalizers. guaranteed to be non-{@code null}.
     * @see #finalizers()
     */
    @JsonProperty("finalizers")
    @NotNull
    public List<String> getFinalizers() {
        return finalizers;
    }

    /**
     * Sets the list of finalizers which should be attached to all resources in this {@code Orbit}. Unlike the
     * constructor for this class, this method does not accept a {@code null} reference.
     *
     * @param finalizers a list of finalizers. must not be a {@code null} reference.
     * @throws NullPointerException if the given finalizers argument is a {@code null} reference.
     */
    @JsonProperty("finalizers")
    public void setFinalizers(@NotNull final List<String> finalizers) {
        this.finalizers = new LinkedList<>(requireNonNull(finalizers));
    }

    /**
     * Fluent accessor for the additional properties which are not explicitly defined by the schema.
     *
     * @return a map of additional properties. guaranteed to be non-{@code null}.
     * @see #getAdditionalProperties()
     */
    @JsonIgnore
    @NotNull
    public Map<String, Object> additionalProperties() {
        return additionalProperties;
    }

    /**
     * Returns the additional properties which are not explicitly defined in the schema. This is handled by the Jackson
     * library using the {@code @JsonAnyGetter} annotation.
     *
     * @return a map of additional properties. guaranteed to be non-{@code null}.
     * @see #additionalProperties()
     */
    @JsonAnyGetter
    @NotNull
    public Map<String, Object> getAdditionalProperties() {
        return additionalProperties;
    }

    /**
     * Sets an additional property not explicitly defined in the schema. This is handled by the Jackson library using
     * the {@code @JsonAnySetter} annotation.
     *
     * @param name the name of the additional property. must not be a {@code null} reference.
     * @param value the value of the additional property. may be a {@code null} reference.
     * @throws NullPointerException if the given name argument is a {@code null} reference.
     */
    @JsonAnySetter
    public void setAdditionalProperty(@NotNull final String name, final Object value) {
        this.additionalProperties.put(requireNonNull(name), value);
    }

    /**
     * Sets the additional properties which are not explicitly defined in the schema. This method is used to replace the
     * entire map of additional properties with the given map. Passing a {@code null} reference will effectively clear
     * the map of additional properties.
     *
     * @param additionalProperties a map of additional properties. may be a {@code null} reference.
     */
    @JsonIgnore
    public void setAdditionalProperties(@Nullable final Map<String, Object> additionalProperties) {
        this.additionalProperties.clear();
        if (additionalProperties != null) {
            this.additionalProperties.putAll(additionalProperties);
        }
    }

    /**
     * Creates a new {@code OrbitSpecBuilder} with the same properties as this {@code OrbitSpec}.
     *
     * @return a new {@code OrbitSpecBuilder}.
     */
    @JsonIgnore
    @Override
    public OrbitSpecBuilder edit() {
        return new OrbitSpecBuilder(this);
    }

    /**
     * Creates a new {@code OrbitSpecBuilder} with the same properties as this {@code OrbitSpec}. This method is
     * equivalent to {@link #edit()}.
     *
     * @return a new {@code OrbitSpecBuilder}.
     * @see #edit()
     */
    @JsonIgnore
    public OrbitSpecBuilder toBuilder() {
        return edit();
    }

    /** {@inheritDoc} */
    @Override
    public boolean equals(@Nullable final Object o) {
        if (this == o) return true;
        if (!(o instanceof OrbitSpec orbitSpec)) return false;
        return Objects.equals(getLifetime(), orbitSpec.getLifetime())
                && Objects.equals(getFinalizers(), orbitSpec.getFinalizers())
                && Objects.equals(getAdditionalProperties(), orbitSpec.getAdditionalProperties());
    }

    /** {@inheritDoc} */
    @Override
    public int hashCode() {
        return Objects.hash(getLifetime(), getFinalizers(), getAdditionalProperties());
    }
}
