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

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Map;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=haproxy")
public class HAProxyService extends CRUDKubernetesDependentResource<Service, HAProxyCapsule> {
    public static final String TEMPLATE = "service.yaml";
    public static final String NAME_SUFFIX = "-haproxy-svc";

    public HAProxyService() {
        super(Service.class);
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
}
