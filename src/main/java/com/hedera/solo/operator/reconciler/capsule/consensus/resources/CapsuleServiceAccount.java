/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ServiceAccount;
import io.fabric8.kubernetes.api.model.ServiceAccountBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleServiceAccount extends CRUDKubernetesDependentResource<ServiceAccount, ConsensusCapsule> {
    public static final String TEMPLATE = "sa.yaml";
    public static final String NAME_SUFFIX = "-sa";

    public CapsuleServiceAccount() {
        super(ServiceAccount.class);
    }

    @Override
    protected ServiceAccount desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final ServiceAccountBuilder sa =
                loadYaml(ServiceAccount.class, getClass(), TEMPLATE).edit();

        String name = primary.getMetadata().getName();
        name = name.length() > (63 - NAME_SUFFIX.length()) ? name.substring(0, 63 - NAME_SUFFIX.length()) : name;

        return sa.editMetadata()
                .withName(String.format("%s%s", name, NAME_SUFFIX))
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .build();
    }
}
