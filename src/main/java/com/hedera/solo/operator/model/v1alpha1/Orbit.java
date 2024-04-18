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

import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.ContainerPort;
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.LabelSelector;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.ObjectReference;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.model.annotation.Group;
import io.fabric8.kubernetes.model.annotation.Plural;
import io.fabric8.kubernetes.model.annotation.Singular;
import io.fabric8.kubernetes.model.annotation.Version;
import io.sundr.builder.annotations.Buildable;
import io.sundr.builder.annotations.BuildableReference;
import javax.annotation.processing.Generated;
import lombok.EqualsAndHashCode;
import lombok.ToString;

@Version(value = "v1alpha1", storage = true, served = true)
@Group("operator.solo.hedera.com")
@Singular("orbit")
@Plural("orbits")
@Generated("io.fabric8.java.generator.CRGeneratorRunner")
@ToString()
@EqualsAndHashCode(callSuper = true)
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
public class Orbit extends io.fabric8.kubernetes.client.CustomResource<OrbitSpec, OrbitStatus> {}
