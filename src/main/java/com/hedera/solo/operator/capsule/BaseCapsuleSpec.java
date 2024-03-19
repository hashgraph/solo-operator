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

package com.hedera.solo.operator.capsule;

import static java.util.Objects.requireNonNull;
import static java.util.Objects.requireNonNullElseGet;

import com.fasterxml.jackson.annotation.JsonAnyGetter;
import com.fasterxml.jackson.annotation.JsonAnySetter;
import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.model.Affinity;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.api.model.Toleration;
import io.smallrye.common.constraint.NotNull;
import java.util.LinkedHashMap;
import java.util.LinkedList;
import java.util.List;
import java.util.Map;
import java.util.Objects;

/**
 * An abstract base class which provides the common fields and logic shared by all {@code CapsuleSpec} implementations.
 *
 * @param <T> the type of the concrete builder object.
 */
@JsonPropertyOrder({"apiVersion", "kind", "metadata", "orbit", "affinity", "nodeSelector", "tolerations"})
public abstract class BaseCapsuleSpec<T> implements Editable<T>, KubernetesResource {

    /** The orbit to which this capsule belongs. */
    @JsonProperty("orbit")
    private String orbit;

    /** The affinity configuration which should be applied to the deployments controlled by this capsule. */
    @JsonProperty("affinity")
    @JsonInclude(JsonInclude.Include.NON_NULL)
    private Affinity affinity;

    /** The node selector which should be applied to the deployments controlled by this capsule. */
    @JsonProperty("nodeSelector")
    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private Map<String, String> nodeSelector;

    /** The tolerations which should be applied to the deployments controlled by this capsule. */
    @JsonProperty("tolerations")
    @JsonInclude(JsonInclude.Include.NON_EMPTY)
    private List<Toleration> tolerations;

    /** Additional properties which are not explicitly defined in the {@code BaseCapsuleSpec} schema. */
    @JsonIgnore
    private final Map<String, Object> additionalProperties = new LinkedHashMap<>();

    /** Creates an empty {@code BaseCapsuleSpec} with default values. */
    protected BaseCapsuleSpec() {
        this.tolerations = new LinkedList<>();
        this.nodeSelector = new LinkedHashMap<>();
    }

    /**
     * Creates a new {@code BaseCapsuleSpec} with the given orbit, affinity, node selector, and tolerations.
     *
     * @param orbit the orbit to which this capsule belongs. must not be {@code null}.
     * @param affinity the affinity configuration which should be applied to the deployments controlled by this capsule.
     *     may be {@code null}.
     * @param nodeSelector the node selector which should be applied to the deployments controlled by this capsule. may
     *     be {@code null}.
     * @param tolerations the tolerations which should be applied to the deployments controlled by this capsule. may be
     *     {@code null}.
     * @throws NullPointerException if the given orbit argument is a {@code null} reference.
     */
    protected BaseCapsuleSpec(
            @NotNull final String orbit,
            final Affinity affinity,
            final Map<String, String> nodeSelector,
            final List<Toleration> tolerations) {
        this.orbit = requireNonNull(orbit);
        this.affinity = affinity;
        this.nodeSelector = new LinkedHashMap<>(requireNonNullElseGet(nodeSelector, LinkedHashMap::new));
        this.tolerations = new LinkedList<>(requireNonNullElseGet(tolerations, LinkedList::new));
    }

    /**
     * Fluent accessor for the orbit to which this capsule belongs. This method is guaranteed to return a
     * non-{@code null} reference.
     *
     * @return the orbit to which this capsule belongs. guaranteed to be non-{@code null}.
     * @see #getOrbit()
     */
    @JsonIgnore
    @NotNull
    public String orbit() {
        return this.orbit;
    }

    /**
     * Returns the orbit to which this capsule belongs. This method is guaranteed to return a non-{@code null}
     * reference.
     *
     * @return the orbit to which this capsule belongs. guaranteed to be non-{@code null}.
     * @see #orbit()
     */
    @JsonProperty("orbit")
    @NotNull
    public String getOrbit() {
        return orbit;
    }

    /**
     * Sets the orbit to which this capsule belongs. The namespace in which this capsule is created must match the
     * namespace specified in the orbit. This method does not allow the orbit to be set to a {@code null} reference.
     *
     * @param orbit the orbit to which this capsule belongs. must not be a {@code null} reference.
     * @throws NullPointerException if the given orbit argument is a {@code null} reference.
     */
    @JsonProperty("orbit")
    public void setOrbit(@NotNull final String orbit) {
        this.orbit = requireNonNull(orbit);
    }

    /**
     * Fluent accessor for the affinity configuration which should be applied to the deployments controlled by this
     * capsule. This method may return a {@code null} reference.
     *
     * @return an {@link Affinity} instance. may be a {@code null} reference.
     */
    @JsonIgnore
    public Affinity affinity() {
        return this.affinity;
    }

    /**
     * Returns the affinity configuration which should be applied to the deployments controlled by this capsule. This
     * method may return a {@code null} reference.
     *
     * @return an {@link Affinity} instance. may be a {@code null} reference.
     */
    @JsonProperty("affinity")
    public Affinity getAffinity() {
        return affinity;
    }

    /**
     * Sets the affinity configuration which should be applied to the deployments controlled by this capsule. This
     * method allows the affinity to be set to a {@code null} reference. If the given affinity is {@code null}, then the
     * affinity will be cleared.
     *
     * @param affinity an {@link Affinity} instance. may be a {@code null} reference.
     */
    @JsonProperty("affinity")
    public void setAffinity(final Affinity affinity) {
        this.affinity = affinity;
    }

    /**
     * Fluent accessor for the node selector which should be applied to the deployments controlled by this capsule. This
     * method is guaranteed to return a non-{@code null} reference, even if the map of node selector labels is empty.
     *
     * @return a map of key-value label pairs. guaranteed to be non-{@code null}.
     * @see #getNodeSelector()
     */
    @JsonIgnore
    @NotNull
    public Map<String, String> nodeSelector() {
        return this.nodeSelector;
    }

    /**
     * Returns the node selector which should be applied to the deployments controlled by this capsule. This method is
     * guaranteed to return a non-{@code null} reference, even if the map of node selector labels is empty.
     *
     * @return a map of key-value label pairs. guaranteed to be non-{@code null}.
     * @see #nodeSelector()
     */
    @JsonProperty("nodeSelector")
    @NotNull
    public Map<String, String> getNodeSelector() {
        return nodeSelector;
    }

    /**
     * Sets the node selector which should be applied to the deployments controlled by this capsule. Unlike the
     * protected constructor, this method does not allow passing a {@code null} for the map reference.
     *
     * <p>It is important to note that the given map is copied into a new map, so changes to the original map will not
     * affect the state of this object. Passing an empty map will effectively clear the nodeSelector.
     *
     * @param nodeSelector a map of key-value label pairs. must not be a {@code null} reference.
     * @throws NullPointerException if the given nodeSelector argument is a {@code null} reference.
     */
    @JsonProperty("nodeSelector")
    public void setNodeSelector(@NotNull final Map<String, String> nodeSelector) {
        this.nodeSelector = new LinkedHashMap<>(requireNonNull(nodeSelector));
    }

    /**
     * Fluent accessor for the tolerations which should be applied to the deployments controlled by this capsule. This
     * method is guaranteed to return a non-{@code null} reference, even if the list of tolerations is empty.
     *
     * @return a list of {@link Toleration} instances. guaranteed to be non-{@code null}.
     * @see #getTolerations()
     */
    @JsonIgnore
    @NotNull
    public List<Toleration> tolerations() {
        return this.tolerations;
    }

    /**
     * Returns the tolerations which should be applied to the deployments controlled by this capsule. This method is
     * guaranteed to return a non-{@code null} reference.
     *
     * @return a list of {@link Toleration} instances. guaranteed to be non-{@code null}, but may be an empty list.
     * @see #tolerations()
     */
    @JsonProperty("tolerations")
    @NotNull
    public List<Toleration> getTolerations() {
        return tolerations;
    }

    /**
     * Sets the tolerations which should be applied to the deployments controlled by this capsule. Unlike the protected
     * constructor, this method does not allow passing a {@code null} for the list reference.
     *
     * <p>It is important to note that the given list of tolerations is copied into a new list, so changes to the
     * original list will not affect the state of this object. Passing an empty list will effectively clear the list of
     * tolerations.
     *
     * @param tolerations a list of {@link Toleration} instances. must not be a {@code null} reference.
     * @throws NullPointerException if the given tolerations argument is a {@code null} reference.
     */
    @JsonProperty("tolerations")
    public void setTolerations(@NotNull final List<Toleration> tolerations) {
        this.tolerations = new LinkedList<>(requireNonNull(tolerations));
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
        return this.additionalProperties;
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
        return this.additionalProperties;
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

    /** {@inheritDoc} */
    @Override
    public boolean equals(final Object o) {
        if (this == o) return true;
        if (!(o instanceof BaseCapsuleSpec<?> that)) return false;
        return Objects.equals(getOrbit(), that.getOrbit())
                && Objects.equals(getAffinity(), that.getAffinity())
                && Objects.equals(getNodeSelector(), that.getNodeSelector())
                && Objects.equals(getTolerations(), that.getTolerations())
                && Objects.equals(getAdditionalProperties(), that.getAdditionalProperties());
    }

    /** {@inheritDoc} */
    @Override
    public int hashCode() {
        return Objects.hash(getOrbit(), getAffinity(), getNodeSelector(), getTolerations(), getAdditionalProperties());
    }
}
