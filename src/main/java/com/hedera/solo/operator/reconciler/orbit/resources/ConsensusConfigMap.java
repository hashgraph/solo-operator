// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.rbac.Role;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Optional;

@KubernetesDependent
public class ConsensusConfigMap extends CRUDNoGCKubernetesDependentResource<ConfigMap, Orbit> implements ResourceDiscriminator<ConfigMap, Orbit> {

    public static final String TEMPLATE = "consensus-config-cm.yaml";
    public static final String NAME = "consensus-config";

    public ConsensusConfigMap() {
        super(ConfigMap.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("consensus-config")
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    protected ConfigMap desired(final Orbit primary, final Context<Orbit> context) {
        ConfigMap cm = context.getSecondaryResource(ConfigMap.class, this).orElse(loadYaml(ConfigMap.class, getClass(), TEMPLATE));

        return edit(cm)
                .with(builder -> Metadata.applyOrbitMetaTo(primary, builder.editMetadata()))
                .with(metadata -> metadata.editMetadata()
                        .withNamespace(primary.getMetadata().getName())
                        .addToLabels("consensus-config", "true"))
                .build();
    }

    @Override
    public Optional<ConfigMap> distinguish(Class<ConfigMap> resource, Orbit primary, Context<Orbit> context) {
        return eventSource().orElseThrow().list(primary.getMetadata().getName()).findAny();
    }
}
