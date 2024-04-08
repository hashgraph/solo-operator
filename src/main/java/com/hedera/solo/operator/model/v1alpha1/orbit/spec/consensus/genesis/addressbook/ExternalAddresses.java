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

package com.hedera.solo.operator.model.v1alpha1.orbit.spec.consensus.genesis.addressbook;

@com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
@com.fasterxml.jackson.annotation.JsonPropertyOrder({
    "externalHostname",
    "externalPort",
    "internalHostname",
    "internalPort",
    "nickName",
    "nodeAccount",
    "nodeId",
    "publicSigningKey",
    "selfName",
    "weight"
})
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
public class ExternalAddresses implements io.fabric8.kubernetes.api.model.KubernetesResource {

    @com.fasterxml.jackson.annotation.JsonProperty("externalHostname")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String externalHostname;

    public String getExternalHostname() {
        return externalHostname;
    }

    public void setExternalHostname(String externalHostname) {
        this.externalHostname = externalHostname;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("externalPort")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Long externalPort;

    public Long getExternalPort() {
        return externalPort;
    }

    public void setExternalPort(Long externalPort) {
        this.externalPort = externalPort;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("internalHostname")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String internalHostname;

    public String getInternalHostname() {
        return internalHostname;
    }

    public void setInternalHostname(String internalHostname) {
        this.internalHostname = internalHostname;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("internalPort")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Long internalPort;

    public Long getInternalPort() {
        return internalPort;
    }

    public void setInternalPort(Long internalPort) {
        this.internalPort = internalPort;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("nickName")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String nickName;

    public String getNickName() {
        return nickName;
    }

    public void setNickName(String nickName) {
        this.nickName = nickName;
    }

    /**
     * In the form `0.0.x`, where `x` is the account number. The Platform SDK address book calls this the `memo` field.
     */
    @com.fasterxml.jackson.annotation.JsonProperty("nodeAccount")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "In the form `0.0.x`, where `x` is the account number. The Platform SDK address book\ncalls this the `memo` field.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String nodeAccount;

    public String getNodeAccount() {
        return nodeAccount;
    }

    public void setNodeAccount(String nodeAccount) {
        this.nodeAccount = nodeAccount;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("nodeId")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String nodeId;

    public String getNodeId() {
        return nodeId;
    }

    public void setNodeId(String nodeId) {
        this.nodeId = nodeId;
    }

    /** The public signing key for the node. This should be in PEM format. */
    @com.fasterxml.jackson.annotation.JsonProperty("publicSigningKey")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The public signing key for the node. This should be in PEM format.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String publicSigningKey;

    public String getPublicSigningKey() {
        return publicSigningKey;
    }

    public void setPublicSigningKey(String publicSigningKey) {
        this.publicSigningKey = publicSigningKey;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("selfName")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String selfName;

    public String getSelfName() {
        return selfName;
    }

    public void setSelfName(String selfName) {
        this.selfName = selfName;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("weight")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Long weight;

    public Long getWeight() {
        return weight;
    }

    public void setWeight(Long weight) {
        this.weight = weight;
    }
}
