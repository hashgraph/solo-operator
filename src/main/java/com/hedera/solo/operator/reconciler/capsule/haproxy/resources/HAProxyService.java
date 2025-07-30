// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_INSTANCE;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Map;

/** The HAProxyService class is a Kubernetes dependent resource that manages the deployment of a HAProxy service. */
@KubernetesDependent
public class HAProxyService extends CRUDKubernetesDependentResource<Service, HAProxyCapsule> {
    public static final String TEMPLATE = "service.yaml";
    public static final String NAME_SUFFIX = "-ha-svc";

    /** Default constructor for HAProxyService, initializes the resource type as Service. */
    public HAProxyService() {
        super(Service.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Service>()
                .withKubernetesDependentInformerConfig(InformerConfiguration.builder(Service.class)
                        .withLabelSelector((Constants.LABEL_SELECTOR_HAPROXY_APP))
                        .build())
                .build());
    }

    @Override
    protected Service desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = metadata.getNamespace();
        final Map<String, String> labels = HAProxyDeployment.prepareHAProxyLabels(primary);

        final Service service = loadYaml(Service.class, getClass(), TEMPLATE);
        return Metadata.applyStandardLabels(
                        service.edit().editMetadata(),
                        validName,
                        "none",
                        Constants.COMPONENT_HAPROXY,
                        Constants.COMPONENT_SERVICE,
                        labels)
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata()
                .editSpec()
                .withSelector(Map.of(
                        "app",
                        "haproxy",
                        LABEL_K8S_INSTANCE,
                        validName(metadata.getName(), HAProxyDeployment.NAME_SUFFIX)))
                .endSpec()
                .build();
    }
}
