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

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyDescription;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import com.fasterxml.jackson.annotation.JsonSetter;
import com.fasterxml.jackson.annotation.JsonValue;
import com.fasterxml.jackson.annotation.Nulls;
import com.fasterxml.jackson.databind.JsonDeserializer;
import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import com.hedera.solo.operator.model.v1alpha1.orbit.spec.Consensus;
import com.hedera.solo.operator.model.v1alpha1.orbit.spec.Telemetry;
import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.ContainerPort;
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.api.model.LabelSelector;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.ObjectReference;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.sundr.builder.annotations.Buildable;
import io.sundr.builder.annotations.BuildableReference;
import java.util.List;
import javax.annotation.processing.Generated;
import lombok.EqualsAndHashCode;
import lombok.ToString;

@JsonInclude(JsonInclude.Include.NON_NULL)
@JsonPropertyOrder({"consensus", "deploymentModel", "finalizers", "lifetime", "telemetry"})
@JsonDeserialize(using = JsonDeserializer.None.class)
@Generated("io.fabric8.java.generator.CRGeneratorRunner")
@ToString()
@EqualsAndHashCode()
@Buildable(
        editableEnabled = false,
        validationEnabled = false,
        generateBuilderPackage = false,
        builderPackage = "io.fabric8.kubernetes.api.builder",
        refs = {
            @BuildableReference(ObjectMeta.class),
            @BuildableReference(ObjectReference.class),
            @BuildableReference(LabelSelector.class),
            @BuildableReference(Container.class),
            @BuildableReference(EnvVar.class),
            @BuildableReference(ContainerPort.class),
            @BuildableReference(Volume.class),
            @BuildableReference(VolumeMount.class)
        })
public class OrbitSpec implements KubernetesResource {

    /** Shared (global) configuration for all `ConsensusCapsules` in the `Orbit`. */
    @JsonProperty("consensus")
    @JsonPropertyDescription("Shared (global) configuration for all `ConsensusCapsules` in the `Orbit`.")
    @JsonSetter(nulls = Nulls.SKIP)
    private Consensus consensus;

    public Consensus getConsensus() {
        return consensus;
    }

    public void setConsensus(Consensus consensus) {
        this.consensus = consensus;
    }

    public enum DeploymentModel {
        @JsonProperty("SingleKubernetesCluster")
        SingleKubernetesCluster("SingleKubernetesCluster"),
        @JsonProperty("MultipleKubernetesClusters")
        MultipleKubernetesClusters("MultipleKubernetesClusters");

        String value;

        DeploymentModel(String value) {
            this.value = value;
        }

        @JsonValue()
        public String getValue() {
            return value;
        }
    }

    /** The deployment model for the `Orbit`. */
    @JsonProperty("deploymentModel")
    @JsonPropertyDescription("The deployment model for the `Orbit`.\n")
    @JsonSetter(nulls = Nulls.SKIP)
    private DeploymentModel deploymentModel;

    public DeploymentModel getDeploymentModel() {
        return deploymentModel;
    }

    public void setDeploymentModel(DeploymentModel deploymentModel) {
        this.deploymentModel = deploymentModel;
    }

    /** A list of finalizers which should be attached to all resources in this `Orbit`. */
    @JsonProperty("finalizers")
    @JsonPropertyDescription("A list of finalizers which should be attached to all resources in this `Orbit`.")
    @JsonSetter(nulls = Nulls.SKIP)
    private java.util.List<String> finalizers;

    public List<String> getFinalizers() {
        return finalizers;
    }

    public void setFinalizers(List<String> finalizers) {
        this.finalizers = finalizers;
    }

    /**
     * The lifetime of the `Orbit`. The string value must be one of the following:
     *
     * <p>- A standard textual Java duration (eg: `P2DT15H`). - A human readable duration (eg: `2 days 15 hours`).
     */
    @JsonProperty("lifetime")
    @JsonPropertyDescription(
            "The lifetime of the `Orbit`. The string value must be one of the following:\n\n- A standard textual Java duration (eg: `P2DT15H`).\n- A human readable duration (eg: `2 days 15 hours`).\n")
    @JsonSetter(nulls = Nulls.SKIP)
    private String lifetime;

    public String getLifetime() {
        return lifetime;
    }

    public void setLifetime(String lifetime) {
        this.lifetime = lifetime;
    }

    @JsonProperty("telemetry")
    @JsonSetter(nulls = Nulls.SKIP)
    private Telemetry telemetry;

    public Telemetry getTelemetry() {
        return telemetry;
    }

    public void setTelemetry(Telemetry telemetry) {
        this.telemetry = telemetry;
    }
}
