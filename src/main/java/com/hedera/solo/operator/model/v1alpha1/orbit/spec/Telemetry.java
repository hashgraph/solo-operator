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

import com.hedera.solo.operator.model.v1alpha1.orbit.spec.telemetry.Logging;
import com.hedera.solo.operator.model.v1alpha1.orbit.spec.telemetry.Metrics;

@com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
@com.fasterxml.jackson.annotation.JsonPropertyOrder({"logging", "metrics"})
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
public class Telemetry implements io.fabric8.kubernetes.api.model.KubernetesResource {

    /** Logging configuration for all members of the `Orbit`. */
    @com.fasterxml.jackson.annotation.JsonProperty("logging")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription("Logging configuration for all members of the `Orbit`.")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Logging logging;

    public Logging getLogging() {
        return logging;
    }

    public void setLogging(Logging logging) {
        this.logging = logging;
    }

    /** Metrics configuration for all members of the `Orbit`. */
    @com.fasterxml.jackson.annotation.JsonProperty("metrics")
    @com.fasterxml.jackson.annotation.JsonPropertyDescription("Metrics configuration for all members of the `Orbit`.")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Metrics metrics;

    public Metrics getMetrics() {
        return metrics;
    }

    public void setMetrics(Metrics metrics) {
        this.metrics = metrics;
    }
}
