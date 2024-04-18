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
import io.fabric8.kubernetes.api.model.rbac.RoleBinding;
import io.fabric8.kubernetes.api.model.rbac.RoleBindingBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleRoleBinding extends CRUDKubernetesDependentResource<RoleBinding, ConsensusCapsule> {

    public static final String TEMPLATE = "role-binding.yaml";
    public static final String NAME_SUFFIX = "-rb";

    public CapsuleRoleBinding() {
        super(RoleBinding.class);
    }

    @Override
    protected RoleBinding desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final RoleBindingBuilder rb =
                loadYaml(RoleBinding.class, getClass(), TEMPLATE).edit();

        String rbName = primary.getMetadata().getName();
        rbName =
                rbName.length() > (63 - NAME_SUFFIX.length()) ? rbName.substring(0, 63 - NAME_SUFFIX.length()) : rbName;

        String saName = primary.getMetadata().getName();
        saName = saName.length() > (63 - CapsuleServiceAccount.NAME_SUFFIX.length())
                ? saName.substring(0, 63 - CapsuleServiceAccount.NAME_SUFFIX.length())
                : saName;

        rb.editMetadata()
                .withName(String.format("%s%s", rbName, NAME_SUFFIX))
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata();

        //        var sb = rb.addNewSubject();
        //        if (rb.hasSubjects()) {
        var sb = rb.editSubject(0);
        //        }

        sb.withName(String.format("%s%s", saName, CapsuleServiceAccount.NAME_SUFFIX))
                .withNamespace(primary.getMetadata().getNamespace())
                .endSubject();

        return rb.build();
    }
}
