// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Optional;

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
}
