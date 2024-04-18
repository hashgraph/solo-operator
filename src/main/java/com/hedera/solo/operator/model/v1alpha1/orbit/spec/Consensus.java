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

package com.hedera.solo.operator.model.v1alpha1.orbit.spec;

import com.hedera.solo.operator.model.v1alpha1.orbit.spec.consensus.Genesis;

@com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
@com.fasterxml.jackson.annotation.JsonPropertyOrder({
    "applicationConfiguration",
    "genesis",
    "networkSettings",
    "provisioningModel"
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
public class Consensus implements io.fabric8.kubernetes.api.model.KubernetesResource {

    /**
     * The application configuration for the `Orbit`. This is equivalent to the `data/config/application.properties`
     * file.
     */
    @com.fasterxml.jackson.annotation.JsonProperty("applicationConfiguration")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The application configuration for the `Orbit`. This is equivalent to the \n`data/config/application.properties` file.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.Map<String, String> applicationConfiguration;

    public java.util.Map<String, String> getApplicationConfiguration() {
        return applicationConfiguration;
    }

    public void setApplicationConfiguration(java.util.Map<String, String> applicationConfiguration) {
        this.applicationConfiguration = applicationConfiguration;
    }

    /**
     * Configuration properties used during the genesis initialization process. Prior to DAB Phase 2, some properties
     * will be used for both genesis and non-genesis initialization. After DAB Phase 2, these properties will only be
     * used during the genesis initialization process.
     */
    @com.fasterxml.jackson.annotation.JsonProperty("genesis")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "Configuration properties used during the genesis initialization process. Prior to DAB Phase 2,\nsome properties will be used for both genesis and non-genesis initialization. After DAB Phase 2,\nthese properties will only be used during the genesis initialization process.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Genesis genesis;

    public Genesis getGenesis() {
        return genesis;
    }

    public void setGenesis(Genesis genesis) {
        this.genesis = genesis;
    }

    /** The network settings for the `Orbit`. This is equivalent to the Platform SDK `settings.txt` file. */
    @com.fasterxml.jackson.annotation.JsonProperty("networkSettings")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The network settings for the `Orbit`. This is equivalent to the Platform SDK `settings.txt` file.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.Map<String, String> networkSettings;

    public java.util.Map<String, String> getNetworkSettings() {
        return networkSettings;
    }

    public void setNetworkSettings(java.util.Map<String, String> networkSettings) {
        this.networkSettings = networkSettings;
    }

    public enum ProvisioningModel {
        @com.fasterxml.jackson.annotation.JsonProperty("InitContainerDinD")
        InitContainerDinD("InitContainerDinD"),
        @com.fasterxml.jackson.annotation.JsonProperty("InitContainerJava")
        InitContainerJava("InitContainerJava"),
        @com.fasterxml.jackson.annotation.JsonProperty("DeterministicContainerImage")
        DeterministicContainerImage("DeterministicContainerImage");

        String value;

        ProvisioningModel(String value) {
            this.value = value;
        }

        @com.fasterxml.jackson.annotation.JsonValue()
        public String getValue() {
            return value;
        }
    }

    /** The deployment model for the `Orbit`. Initially the operator will support the `InitContainerJava` model. */
    @com.fasterxml.jackson.annotation.JsonProperty("provisioningModel")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription(
            "The deployment model for the `Orbit`. Initially the operator will support the `InitContainerJava` model.\n")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private ProvisioningModel provisioningModel;

    public ProvisioningModel getProvisioningModel() {
        return provisioningModel;
    }

    public void setProvisioningModel(ProvisioningModel provisioningModel) {
        this.provisioningModel = provisioningModel;
    }
}
