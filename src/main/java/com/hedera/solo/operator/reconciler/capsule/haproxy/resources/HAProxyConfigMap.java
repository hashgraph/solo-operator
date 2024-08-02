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

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.function.BiFunction;

@KubernetesDependent(labelSelector = "app=haproxy")
public class HAProxyConfigMap extends CRUDKubernetesDependentResource<ConfigMap, HAProxyCapsule> {
    public static final String TEMPLATE = "configmap.yaml";
    public static final String NAME_SUFFIX = "-ha-cm";

    public HAProxyConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final Base64.Decoder decoder = Base64.getDecoder();
        BiFunction<Secret, String, String> get =
                (secret, name) -> new String(decoder.decode(secret.getData().get(name)), StandardCharsets.UTF_8);
        String pem = context.getSecondaryResource(Secret.class)
                .map(secret -> get.apply(secret, "hedera.key").trim() + "\n"
                        + get.apply(secret, "hedera.crt").trim() + "\n")
                .orElse(null);

        return loadYaml(ConfigMap.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .addToData("hedera.pem", pem)
                .build();
    }
}
