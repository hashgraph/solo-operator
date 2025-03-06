// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.render.RenderUtils.join;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Objects;
import java.util.Optional;

@KubernetesDependent(labelSelector = "app=consensus-node", resourceDiscriminator = CapsuleConfigMap.class)
public class CapsuleConfigMap extends CRUDKubernetesDependentResource<ConfigMap, ConsensusCapsule>
        implements ResourceDiscriminator<ConfigMap, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-cm";

    public CapsuleConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        return edit(context.getSecondaryResource(ConfigMap.class, this).orElseGet(ConfigMap::new))
                .with(metadata -> metadata.editMetadata()
                        .withName(validName(Metadata.prepareNodeName(primary), NAME_SUFFIX))
                        .withNamespace(primary.getMetadata().getNamespace())
                        .addToLabels("app", "consensus-node"))
                .with(data -> data.addToData("log4j2.xml", primary.getSpec().getLog4j2_xml())
                        .addToData("nodeProperties", join(primary.getSpec().getNodeProperties(), "=")))
                .build();
    }

    @Override
    public Optional<ConfigMap> distinguish(
            Class<ConfigMap> resource, ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        String name = validName(Metadata.prepareNodeName(primary), NAME_SUFFIX);
        return context.eventSourceRetriever().getResourceEventSourcesFor(ConfigMap.class).stream()
                .flatMap(es -> es.getSecondaryResources(primary).stream())
                .filter(cm -> Objects.equals(name, cm.getMetadata().getName()))
                .findAny();
    }
}
