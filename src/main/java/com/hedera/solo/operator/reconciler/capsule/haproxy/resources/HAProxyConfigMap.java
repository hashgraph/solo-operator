// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Map;

/** The {@code HAProxyConfigMap} class represents a Kubernetes ConfigMap resource for HAProxy capsule configuration. */
@KubernetesDependent
public class HAProxyConfigMap extends CRUDKubernetesDependentResource<ConfigMap, HAProxyCapsule> {
    public static final String TEMPLATE = "configmap.yaml";
    public static final String NAME_SUFFIX = "-ha-cm";

    /** Constructs a new {@code HAProxyConfigMap} object. */
    public HAProxyConfigMap() {
        super(ConfigMap.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withKubernetesDependentInformerConfig(InformerConfiguration.builder(ConfigMap.class)
                        .withLabelSelector(Constants.LABEL_SELECTOR_HAPROXY_APP)
                        .build())
                .build());
    }

    @Override
    protected ConfigMap desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = metadata.getNamespace();
        final ConfigMap cm = loadYaml(ConfigMap.class, getClass(), TEMPLATE);
        final Map<String, String> labels = HAProxyDeployment.prepareHAProxyLabels(primary);

        return Metadata.applyStandardLabels(
                        cm.edit().editMetadata(),
                        validName,
                        "none",
                        Constants.COMPONENT_HAPROXY,
                        Constants.COMPONENT_CONFIG_MAP,
                        labels)
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata()
                .build();
    }
}
