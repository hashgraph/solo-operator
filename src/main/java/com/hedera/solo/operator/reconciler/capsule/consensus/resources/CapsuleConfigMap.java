// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent(labelSelector = "app=consensus-node")
public class CapsuleConfigMap extends CRUDKubernetesDependentResource<ConfigMap, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-cm";

    public CapsuleConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        return edit(context.getSecondaryResource(ConfigMap.class).orElseGet(ConfigMap::new))
                .with(metadata -> metadata.editMetadata()
                        .withName(validName(Metadata.prepareNodeName(primary), NAME_SUFFIX))
                        .withNamespace(primary.getMetadata().getNamespace())
                        .addToLabels("app", "consensus-node"))
                .with(data -> data.addToData("log4j2.xml", primary.getSpec().getLog4j2_xml()))
                .build();
    }
}
