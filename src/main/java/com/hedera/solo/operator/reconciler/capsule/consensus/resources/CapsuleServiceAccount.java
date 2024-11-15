// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.ServiceAccount;
import io.fabric8.kubernetes.api.model.ServiceAccountBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;

@KubernetesDependent
public class CapsuleServiceAccount extends CRUDKubernetesDependentResource<ServiceAccount, ConsensusCapsule> {
    public static final String TEMPLATE = "sa.yaml";
    public static final String NAME_SUFFIX = "-sa";

    public CapsuleServiceAccount() {
        super(ServiceAccount.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ServiceAccount>()
                .build());
    }

    @Override
    protected ServiceAccount desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final ServiceAccountBuilder sa =
                loadYaml(ServiceAccount.class, getClass(), TEMPLATE).edit();

        String name = Metadata.prepareNodeName(primary);
        name = name.length() > (63 - NAME_SUFFIX.length()) ? name.substring(0, 63 - NAME_SUFFIX.length()) : name;

        return sa.editMetadata()
                .withName(String.format("%s%s", name, NAME_SUFFIX))
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .build();
    }
}
