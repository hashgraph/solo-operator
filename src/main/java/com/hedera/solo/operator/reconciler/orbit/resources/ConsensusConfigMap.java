// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.library.render.RenderUtils.join;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.Consensus;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.consensus.genesis.AddressBook;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.LinkedHashMap;
import java.util.Map;

@KubernetesDependent
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
        cmb.addToData("settings.txt", join(primary.getSpec().getConsensus().getNetworkSettings(), ","));
        cmb.addToData("application.properties", join(applicationConfiguration(primary), "="));
        cmb.addToData(
                "api-permission.properties",
                join(primary.getSpec().getConsensus().getPermissionConfiguration(), "="));
        cmb.addToData(
                "genesis.properties", join(primary.getSpec().getConsensus().getGenesisProperties(), "="));
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

    Map<String, String> applicationConfiguration(Orbit orbit) {
        Consensus consensus = orbit.getSpec().getConsensus();
        Map<String, String> applicationConfiguration = new LinkedHashMap<>(consensus.getApplicationConfiguration());
        AddressBook addressBook = orbit.getSpec().getConsensus().getGenesis().getAddressBook();
        applicationConfiguration.put("hedera.realm", addressBook.getRealmId().toString());
        applicationConfiguration.put("hedera.shard", addressBook.getShardId().toString());
        return applicationConfiguration;
    }
}
