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

package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.library.resource.SystemResource.getSystemResourceAsString;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitSpec;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.Consensus;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Function;
import java.util.stream.Collectors;

@KubernetesDependent(resourceDiscriminator = ConsensusConfigMap.Discriminator.class)
public class ConsensusConfigMap extends CRUDNoGCKubernetesDependentResource<ConfigMap, Orbit> {

    public static final String TEMPLATE = "consensus-config-cm.yaml";
    public static final String NAME = "consensus-config";

    public ConsensusConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(final Orbit primary, final Context<Orbit> context) {
        final ConfigMapBuilder cm =
                loadYaml(ConfigMap.class, getClass(), TEMPLATE).edit();

        // questions:
        // 1. Are bootstrap.properties a copy of application.properties?
        // 2. Do we need api-permission.properties?
        cm.addToData("settings.txt", get(primary, "hedera/mainnet/settings.txt", Consensus::getNetworkSettings, ","));
        cm.addToData("application.properties", get(primary, "hedera/mainnet/application.properties", Consensus::getApplicationConfiguration, "="));
        cm.addToData("bootstrap.properties", cm.getData().get("application.properties"));
//        cm.addToData("api-permission.properties", get(primary, "hedera/mainnet/api-permission.properties", $ -> null, "="));

        return Metadata.applyOrbitMetaTo(primary, cm.editMetadata())
                .withNamespace(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }

    public static class Discriminator implements ResourceDiscriminator<ConfigMap, Orbit> {
        @Override
        public Optional<ConfigMap> distinguish(
                final Class<ConfigMap> resource, final Orbit primary, final Context<Orbit> context) {
            return Optional.ofNullable(context.getClient()
                    .configMaps()
                    .inNamespace(primary.getMetadata().getName())
                    .withName(NAME)
                    .get());
        }
    }

    String get(final Orbit primary, final String resource, final Function<Consensus, Map<String, String>> configMap, final String delimiter) {
        Optional<Map<String, String>> networkSetting = Optional.ofNullable(primary).map(CustomResource::getSpec).map(OrbitSpec::getConsensus).map(configMap);
        if (networkSetting.isPresent()) {
            return networkSetting.get().entrySet().stream().map(e -> e.getKey() + delimiter + e.getValue()).collect(Collectors.joining("\n","", "\n"));
        } else {
            try {
                return getSystemResourceAsString(resource);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        }
    }
}
