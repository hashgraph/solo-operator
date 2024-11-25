// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.library.resource.SystemResource.getSystemResourceAsStringUnchecked;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitSpec;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.Consensus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Map;
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
        final String orbitName = primary.getMetadata().getName();
        final ConfigMap cm = loadYaml(ConfigMap.class, getClass(), TEMPLATE);
        final ConfigMapBuilder cmb =
                Metadata.applyOrbitMetaTo(primary, cm.edit().editMetadata()).endMetadata();
        cmb.addToData(
                "settings.txt", get(primary, "import/hedera/mainnet/settings.txt", Consensus::getNetworkSettings, ","));
        cmb.addToData(
                "application.properties",
                get(
                        primary,
                        "import/hedera/mainnet/application.properties",
                        Consensus::getApplicationConfiguration,
                        "="));
        cmb.addToData(
                "api-permission.properties",
                get(
                        primary,
                        "import/hedera/mainnet/api-permission.properties",
                        Consensus::getPermissionConfiguration,
                        "="));
        return Metadata.applyStandardLabels(
                        cmb.editMetadata(),
                        NAME,
                        orbitName,
                        Constants.COMPONENT_ORBIT,
                        Constants.COMPONENT_CONFIG_MAP,
                        null)
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

    String get(
            final Orbit primary,
            final String resource,
            final Function<Consensus, Map<String, String>> configMap,
            final String delimiter) {
        Optional<Map<String, String>> networkSetting = Optional.ofNullable(primary)
                .map(CustomResource::getSpec)
                .map(OrbitSpec::getConsensus)
                .map(configMap);
        if (networkSetting.isPresent()) {
            return networkSetting.get().entrySet().stream()
                    .map(e -> e.getKey() + delimiter + e.getValue())
                    .collect(Collectors.joining("\n", "", "\n"));
        } else {
            return getSystemResourceAsStringUnchecked(resource);
        }
    }
}
