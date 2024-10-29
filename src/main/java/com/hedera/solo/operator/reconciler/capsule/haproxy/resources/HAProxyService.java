// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
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
                .withLabelSelector(LABEL_SELECTOR_HAPROXY)
                .build());
    }

    @Override
    protected Service desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);

        return loadYaml(Service.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .editSpec()
                .withSelector(Map.of(
                        "app",
                        "haproxy",
                        "consensus-capsule",
                        primary.getSpec().getConsensus().getNode()))
                .endSpec()
                .build();
    }

    @Override
    public Optional<Service> distinguish(
            Class<Service> resource, HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        return context.getSecondaryResources(Service.class).stream()
                .filter(svc -> "haproxy".equals(svc.getMetadata().getLabels().get("app")))
                .findAny();
    }
}
