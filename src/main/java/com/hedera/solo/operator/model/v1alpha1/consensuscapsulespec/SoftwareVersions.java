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

package com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec;

import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.model.KubernetesResource;

@com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
@com.fasterxml.jackson.annotation.JsonPropertyOrder({"consensusNode", "openTelemetryCollector", "streamUploader"})
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
public class SoftwareVersions implements Editable<SoftwareVersionsBuilder>, KubernetesResource {

    @com.fasterxml.jackson.annotation.JsonProperty("consensusNode")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private com.hedera.solo.operator.model.common.SoftwareVersion consensusNode;

    public com.hedera.solo.operator.model.common.SoftwareVersion getConsensusNode() {
        return consensusNode;
    }

    public void setConsensusNode(com.hedera.solo.operator.model.common.SoftwareVersion consensusNode) {
        this.consensusNode = consensusNode;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("openTelemetryCollector")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private com.hedera.solo.operator.model.common.SoftwareVersion openTelemetryCollector;

    public com.hedera.solo.operator.model.common.SoftwareVersion getOpenTelemetryCollector() {
        return openTelemetryCollector;
    }

    public void setOpenTelemetryCollector(
            com.hedera.solo.operator.model.common.SoftwareVersion openTelemetryCollector) {
        this.openTelemetryCollector = openTelemetryCollector;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("streamUploader")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private com.hedera.solo.operator.model.common.SoftwareVersion streamUploader;

    public com.hedera.solo.operator.model.common.SoftwareVersion getStreamUploader() {
        return streamUploader;
    }

    public void setStreamUploader(com.hedera.solo.operator.model.common.SoftwareVersion streamUploader) {
        this.streamUploader = streamUploader;
    }

    @Override
    public SoftwareVersionsBuilder edit() {
        return new SoftwareVersionsBuilder(this);
    }
}
