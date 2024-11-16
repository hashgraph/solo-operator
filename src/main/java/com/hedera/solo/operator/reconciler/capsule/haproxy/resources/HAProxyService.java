// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler.HAS_LABEL_SELECTOR_HAPROXY;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Map;
import java.util.Optional;

@KubernetesDependent
public class HAProxyService extends CRUDKubernetesDependentResource<Service, HAProxyCapsule>
        implements ResourceDiscriminator<Service, HAProxyCapsule> {
    public static final String TEMPLATE = "service.yaml";
    public static final String NAME_SUFFIX = "-ha-svc";

    public HAProxyService() {
        super(Service.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Service>()
                .withUseSSA(Boolean.FALSE)
                .withResourceDiscriminator(this)
                .withLabelSelector(Constants.LABEL_SELECTOR_HAPROXY)
                .build());
    }

    @Override
    protected Service desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = metadata.getNamespace();
        final String orbit = primary.getSpec().getOrbit();
        final Map<String, String> labels = HAProxyDeployment.prepareHAProxyLabels(primary);

        final Service service = loadYaml(Service.class, getClass(), TEMPLATE);
        return Metadata.applyStandardLabels(
                        service.edit().editMetadata(),
                        validName,
                        orbit,
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
                        Constants.LABEL_NODE_NAME,
                        Metadata.prepareNodeName(primary.getSpec().getNodeId())))
                .endSpec()
                .build();
    }

    @Override
    public Optional<Service> distinguish(
            Class<Service> resource, HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        return context.getSecondaryResources(Service.class).stream()
                .filter(HAS_LABEL_SELECTOR_HAPROXY)
                .findAny();
    }
}
