// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.NamespaceBuilder;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Optional;

@KubernetesDependent
public class OrbitNamespace extends CRUDKubernetesDependentResource<Namespace, Orbit> /*implements ResourceDiscriminator<Namespace, Orbit>*/ {
    public static final String TEMPLATE = "namespace.yaml";

    public OrbitNamespace() {
        super(Namespace.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Namespace>()
//                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    protected Namespace desired(final Orbit primary, final Context<Orbit> context) {
        final NamespaceBuilder builder =
                loadYaml(Namespace.class, getClass(), TEMPLATE).edit();

        return Metadata.applyOrbitMetaTo(primary, builder.editMetadata())
                .withName(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }

//    @Override
//    protected InformerConfiguration.InformerConfigurationBuilder<Namespace> informerConfigurationBuilder() {
//        return super.informerConfigurationBuilder();
//    }
//
//    @Override
//    public Optional<Namespace> distinguish(Class<Namespace> resource, Orbit primary, Context<Orbit> context) {
//        // use event source if it exists? else:
//        return Optional.ofNullable(context.getClient().namespaces().withName(primary.getMetadata().getName()).get());
//    }
}
