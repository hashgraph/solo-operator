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

package com.hedera.solo.operator.model.v1alpha1;

import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.javaoperatorsdk.operator.api.ObservedGenerationAware;

@com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
@com.fasterxml.jackson.annotation.JsonPropertyOrder({"addressBookEntry", "conditions", "observedGeneration", "phase"})
@com.fasterxml.jackson.databind.annotation.JsonDeserialize(
        using = com.fasterxml.jackson.databind.JsonDeserializer.None.class)
@javax.annotation.processing.Generated("io.fabric8.java.generator.CRGeneratorRunner")
@lombok.ToString()
@lombok.EqualsAndHashCode()
@io.sundr.builder.annotations.Buildable(
        editableEnabled = false,
        validationEnabled = false,
        generateBuilderPackage = false,
        builderPackage = "io.fabric8.kubernetes.api.builder",
        refs = {
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.ObjectMeta.class),
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.ObjectReference.class),
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.LabelSelector.class),
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.Container.class),
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.EnvVar.class),
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.ContainerPort.class),
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.Volume.class),
            @io.sundr.builder.annotations.BuildableReference(io.fabric8.kubernetes.api.model.VolumeMount.class)
        })
public class ConsensusCapsuleStatus
        implements Editable<ConsensusCapsuleStatusBuilder>, KubernetesResource, ObservedGenerationAware {

    /**
     * The address book entry for the `ConsensusCapsule`. This is the entry that will be used by the `Orbit` to
     * communicate with the `ConsensusCapsule`.
     */
    @com.fasterxml.jackson.annotation.JsonProperty("addressBookEntry")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The address book entry for the `ConsensusCapsule`. This is the entry that will be used by the `Orbit` to\ncommunicate with the `ConsensusCapsule`.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private com.hedera.solo.operator.model.common.AddressBookEntry addressBookEntry;

    public com.hedera.solo.operator.model.common.AddressBookEntry getAddressBookEntry() {
        return addressBookEntry;
    }

    public void setAddressBookEntry(com.hedera.solo.operator.model.common.AddressBookEntry addressBookEntry) {
        this.addressBookEntry = addressBookEntry;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("conditions")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.List<com.hedera.solo.operator.model.common.Condition> conditions;

    public java.util.List<com.hedera.solo.operator.model.common.Condition> getConditions() {
        return conditions;
    }

    public void setConditions(java.util.List<com.hedera.solo.operator.model.common.Condition> conditions) {
        this.conditions = conditions;
    }

    /** The generation observed by the operator. */
    @com.fasterxml.jackson.annotation.JsonProperty("observedGeneration")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription("The generation observed by the operator.")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Long observedGeneration;

    @Override
    public Long getObservedGeneration() {
        return observedGeneration;
    }

    @Override
    public void setObservedGeneration(Long observedGeneration) {
        this.observedGeneration = observedGeneration;
    }

    public enum Phase {
        @com.fasterxml.jackson.annotation.JsonProperty("Pending")
        Pending("Pending"),
        @com.fasterxml.jackson.annotation.JsonProperty("Running")
        Running("Running"),
        @com.fasterxml.jackson.annotation.JsonProperty("Succeeded")
        Succeeded("Succeeded"),
        @com.fasterxml.jackson.annotation.JsonProperty("Failed")
        Failed("Failed"),
        @com.fasterxml.jackson.annotation.JsonProperty("Unknown")
        Unknown("Unknown");

        String value;

        Phase(String value) {
            this.value = value;
        }

        @com.fasterxml.jackson.annotation.JsonValue()
        public String getValue() {
            return value;
        }
    }

    /** The current phase of the `ConsensusCapsule`. */
    @com.fasterxml.jackson.annotation.JsonProperty("phase")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription("The current phase of the `ConsensusCapsule`.")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Phase phase;

    public Phase getPhase() {
        return phase;
    }

    public void setPhase(Phase phase) {
        this.phase = phase;
    }

    @Override
    public ConsensusCapsuleStatusBuilder edit() {
        return new ConsensusCapsuleStatusBuilder(this);
    }
}
