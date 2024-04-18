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

@com.fasterxml.jackson.annotation.JsonInclude(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL)
@com.fasterxml.jackson.annotation.JsonPropertyOrder({
    "affinity",
    "imagePullPolicy",
    "imagePullSecrets",
    "nodeSelector",
    "preemptionPolicy",
    "priority",
    "priorityClassName",
    "resources",
    "securityContext",
    "serviceAccountName",
    "softwareVersions",
    "terminationGracePeriodSeconds",
    "tolerations",
    "topologySpreadConstraints"
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
public class ConsensusCapsuleSpec implements Editable<ConsensusCapsuleSpecBuilder>, KubernetesResource {

    @com.fasterxml.jackson.annotation.JsonProperty("affinity")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private io.fabric8.kubernetes.api.model.Affinity affinity;

    public io.fabric8.kubernetes.api.model.Affinity getAffinity() {
        return affinity;
    }

    public void setAffinity(io.fabric8.kubernetes.api.model.Affinity affinity) {
        this.affinity = affinity;
    }

    public enum ImagePullPolicy {
        @com.fasterxml.jackson.annotation.JsonProperty("Always")
        Always("Always"),
        @com.fasterxml.jackson.annotation.JsonProperty("Never")
        Never("Never"),
        @com.fasterxml.jackson.annotation.JsonProperty("IfNotPresent")
        IfNotPresent("IfNotPresent");

        String value;

        ImagePullPolicy(String value) {
            this.value = value;
        }

        @com.fasterxml.jackson.annotation.JsonValue()
        public String getValue() {
            return value;
        }
    }

    @com.fasterxml.jackson.annotation.JsonProperty("imagePullPolicy")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private ImagePullPolicy imagePullPolicy;

    public ImagePullPolicy getImagePullPolicy() {
        return imagePullPolicy;
    }

    public void setImagePullPolicy(ImagePullPolicy imagePullPolicy) {
        this.imagePullPolicy = imagePullPolicy;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("imagePullSecrets")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.List<io.fabric8.kubernetes.api.model.LocalObjectReference> imagePullSecrets;

    public java.util.List<io.fabric8.kubernetes.api.model.LocalObjectReference> getImagePullSecrets() {
        return imagePullSecrets;
    }

    public void setImagePullSecrets(
            java.util.List<io.fabric8.kubernetes.api.model.LocalObjectReference> imagePullSecrets) {
        this.imagePullSecrets = imagePullSecrets;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("nodeSelector")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.Map<String, String> nodeSelector;

    public java.util.Map<String, String> getNodeSelector() {
        return nodeSelector;
    }

    public void setNodeSelector(java.util.Map<String, String> nodeSelector) {
        this.nodeSelector = nodeSelector;
    }

    public enum PreemptionPolicy {
        @com.fasterxml.jackson.annotation.JsonProperty("Never")
        Never("Never"),
        @com.fasterxml.jackson.annotation.JsonProperty("PreemptLowerPriority")
        PreemptLowerPriority("PreemptLowerPriority");

        String value;

        PreemptionPolicy(String value) {
            this.value = value;
        }

        @com.fasterxml.jackson.annotation.JsonValue()
        public String getValue() {
            return value;
        }
    }

    @com.fasterxml.jackson.annotation.JsonProperty("preemptionPolicy")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private PreemptionPolicy preemptionPolicy;

    public PreemptionPolicy getPreemptionPolicy() {
        return preemptionPolicy;
    }

    public void setPreemptionPolicy(PreemptionPolicy preemptionPolicy) {
        this.preemptionPolicy = preemptionPolicy;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("priority")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Long priority;

    public Long getPriority() {
        return priority;
    }

    public void setPriority(Long priority) {
        this.priority = priority;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("priorityClassName")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String priorityClassName;

    public String getPriorityClassName() {
        return priorityClassName;
    }

    public void setPriorityClassName(String priorityClassName) {
        this.priorityClassName = priorityClassName;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("resources")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private io.fabric8.kubernetes.api.model.ResourceRequirements resources;

    public io.fabric8.kubernetes.api.model.ResourceRequirements getResources() {
        return resources;
    }

    public void setResources(io.fabric8.kubernetes.api.model.ResourceRequirements resources) {
        this.resources = resources;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("securityContext")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private io.fabric8.kubernetes.api.model.PodSecurityContext securityContext;

    public io.fabric8.kubernetes.api.model.PodSecurityContext getSecurityContext() {
        return securityContext;
    }

    public void setSecurityContext(io.fabric8.kubernetes.api.model.PodSecurityContext securityContext) {
        this.securityContext = securityContext;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("serviceAccountName")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private String serviceAccountName;

    public String getServiceAccountName() {
        return serviceAccountName;
    }

    public void setServiceAccountName(String serviceAccountName) {
        this.serviceAccountName = serviceAccountName;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("softwareVersions")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.SoftwareVersions softwareVersions;

    public com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.SoftwareVersions getSoftwareVersions() {
        return softwareVersions;
    }

    public void setSoftwareVersions(
            com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.SoftwareVersions softwareVersions) {
        this.softwareVersions = softwareVersions;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("terminationGracePeriodSeconds")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private Long terminationGracePeriodSeconds;

    public Long getTerminationGracePeriodSeconds() {
        return terminationGracePeriodSeconds;
    }

    public void setTerminationGracePeriodSeconds(Long terminationGracePeriodSeconds) {
        this.terminationGracePeriodSeconds = terminationGracePeriodSeconds;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("tolerations")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.List<io.fabric8.kubernetes.api.model.Toleration> tolerations;

    public java.util.List<io.fabric8.kubernetes.api.model.Toleration> getTolerations() {
        return tolerations;
    }

    public void setTolerations(java.util.List<io.fabric8.kubernetes.api.model.Toleration> tolerations) {
        this.tolerations = tolerations;
    }

    @com.fasterxml.jackson.annotation.JsonProperty("topologySpreadConstraints")
    @com.fasterxml.jackson.annotation.JsonSetter(nulls = com.fasterxml.jackson.annotation.Nulls.SKIP)
    private java.util.List<io.fabric8.kubernetes.api.model.TopologySpreadConstraint> topologySpreadConstraints;

    public java.util.List<io.fabric8.kubernetes.api.model.TopologySpreadConstraint> getTopologySpreadConstraints() {
        return topologySpreadConstraints;
    }

    public void setTopologySpreadConstraints(
            java.util.List<io.fabric8.kubernetes.api.model.TopologySpreadConstraint> topologySpreadConstraints) {
        this.topologySpreadConstraints = topologySpreadConstraints;
    }

    @Override
    public ConsensusCapsuleSpecBuilder edit() {
        return new ConsensusCapsuleSpecBuilder(this);
    }
}
