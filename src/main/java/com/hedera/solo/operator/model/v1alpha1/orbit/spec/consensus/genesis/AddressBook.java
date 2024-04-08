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

package com.hedera.solo.operator.model.v1alpha1.orbit.spec.consensus.genesis;

import com.hedera.solo.operator.model.v1alpha1.orbit.spec.consensus.genesis.addressbook.ExternalAddresses;

@com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
@com.fasterxml.jackson.annotation.JsonPropertyOrder({"externalAddresses", "ledgerId", "nextNodeId"})
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
public class AddressBook implements io.fabric8.kubernetes.api.model.KubernetesResource {

    /**
     * The list of external `OrbitCapsules` and physical consensus nodes not managed by this operator deployment aka
     * `Orbit`.
     *
     * <p>The operator should update the configmap deployed when this field changes.
     */
    @com.fasterxml.jackson.annotation.JsonProperty("externalAddresses")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The list of external `OrbitCapsules` and physical consensus nodes not managed by this \noperator deployment aka `Orbit`.\n\nThe operator should update the configmap deployed when this field changes.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.List<ExternalAddresses> externalAddresses;

    public java.util.List<ExternalAddresses> getExternalAddresses() {
        return externalAddresses;
    }

    public void setExternalAddresses(java.util.List<ExternalAddresses> externalAddresses) {
        this.externalAddresses = externalAddresses;
    }

    /**
     * The ledger ID to use. Equivalent to the `swirld` entry in the Platform SDK `config.txt` file format. May be
     * removed in the DAB Phase 3 refactors. The ledger ID should never change once specified.
     */
    @com.fasterxml.jackson.annotation.JsonProperty("ledgerId")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The ledger ID to use. Equivalent to the `swirld` entry in the Platform SDK `config.txt`\nfile format. May be removed in the DAB Phase 3 refactors. The ledger ID should never change once\nspecified.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String ledgerId;

    public String getLedgerId() {
        return ledgerId;
    }

    public void setLedgerId(String ledgerId) {
        this.ledgerId = ledgerId;
    }

    /**
     * The next node ID to use. Equivalent to the `nextNodeId` entry in the Platform SDK `config.txt` file format. May
     * be removed in the DAB Phase 2/3 refactors. The next node ID should never change once specified.
     */
    @com.fasterxml.jackson.annotation.JsonProperty("nextNodeId")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The next node ID to use. Equivalent to the `nextNodeId` entry in the Platform SDK `config.txt`\nfile format. May be removed in the DAB Phase 2/3 refactors. The next node ID should never change\nonce specified.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String nextNodeId;

    public String getNextNodeId() {
        return nextNodeId;
    }

    public void setNextNodeId(String nextNodeId) {
        this.nextNodeId = nextNodeId;
    }
}
