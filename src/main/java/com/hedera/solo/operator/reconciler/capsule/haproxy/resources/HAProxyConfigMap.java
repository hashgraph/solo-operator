// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;

@KubernetesDependent
public class HAProxyConfigMap extends CRUDKubernetesDependentResource<ConfigMap, HAProxyCapsule> {
    public static final String TEMPLATE = "configmap.yaml";
    public static final String NAME_SUFFIX = "-ha-cm";

    public HAProxyConfigMap() {
        super(ConfigMap.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector(LABEL_SELECTOR_HAPROXY)
                .build());
    }

    @Override
    protected ConfigMap desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);

        return loadYaml(ConfigMap.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .build();
    }
}
