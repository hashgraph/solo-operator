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

package com.hedera.solo.operator.model.common;

import static java.util.Objects.requireNonNull;

import com.fasterxml.jackson.annotation.JsonAnyGetter;
import com.fasterxml.jackson.annotation.JsonAnySetter;
import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.smallrye.common.constraint.NotNull;
import io.smallrye.common.constraint.Nullable;
import io.sundr.builder.annotations.Buildable;
import java.util.LinkedHashMap;
import java.util.Map;
import lombok.EqualsAndHashCode;
import lombok.ToString;

@JsonPropertyOrder({"apiVersion", "kind", "metadata", "lastTransitionTime", "message", "reason", "status", "type"})
@JsonInclude(JsonInclude.Include.NON_NULL)
@JsonDeserialize
@ToString
@EqualsAndHashCode
@Buildable(editableEnabled = false, builderPackage = "io.fabric8.kubernetes.api.builder")
public class Condition implements Editable<ConditionBuilder>, KubernetesResource {

    /** The time at which the condition last transitioned from one status to another. */
    @JsonProperty("lastTransitionTime")
    private String lastTransitionTime;

    /** A human-readable message indicating details about the transition. */
    @JsonProperty("message")
    private String message;

    /** The reason for the condition's last transition. */
    @JsonProperty("reason")
    private String reason;

    /** The status of the condition. Must be one of True, False, Unknown. */
    @JsonProperty("status")
    private String status;

    /** The type of the condition. */
    @JsonProperty("type")
    private String type;

    /** Additional properties which are not explicitly defined in the schema. */
    @JsonIgnore
    private final Map<String, Object> additionalProperties = new LinkedHashMap<>();

    public Condition() {}

    public Condition(
            final String lastTransitionTime,
            final String message,
            final String reason,
            final String status,
            final String type) {
        this.lastTransitionTime = lastTransitionTime;
        this.message = message;
        this.reason = reason;
        this.status = status;
        this.type = type;
    }

    @JsonProperty("lastTransitionTime")
    @Nullable
    public String getLastTransitionTime() {
        return lastTransitionTime;
    }

    @JsonProperty("lastTransitionTime")
    public void setLastTransitionTime(@Nullable final String lastTransitionTime) {
        this.lastTransitionTime = lastTransitionTime;
    }

    @JsonProperty("message")
    @Nullable
    public String getMessage() {
        return message;
    }

    @JsonProperty("message")
    public void setMessage(@Nullable final String message) {
        this.message = message;
    }

    @JsonProperty("reason")
    @Nullable
    public String getReason() {
        return reason;
    }

    @JsonProperty("reason")
    public void setReason(@Nullable final String reason) {
        this.reason = reason;
    }

    @JsonProperty("status")
    @Nullable
    public String getStatus() {
        return status;
    }

    @JsonProperty("status")
    public void setStatus(@Nullable final String status) {
        this.status = status;
    }

    @JsonProperty("type")
    @Nullable
    public String getType() {
        return type;
    }

    @JsonProperty("type")
    public void setType(@Nullable final String type) {
        this.type = type;
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

    @Override
    public ConditionBuilder edit() {
        return new ConditionBuilder(this);
    }
}
