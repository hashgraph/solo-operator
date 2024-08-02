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
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=haproxy")
public class HAProxyDeployment extends CRUDKubernetesDependentResource<Deployment, HAProxyCapsule> {
    public static final String TEMPLATE = "deployment.yaml";
    public static final String NAME_SUFFIX = "-ha-deploy";

    public HAProxyDeployment() {
        super(Deployment.class);
    }

    @Override
    protected Deployment desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);

        return loadYaml(Deployment.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .addToLabels(
                        "consensus-capsule", primary.getSpec().getConsensus().getNode())
                .addToLabels("haproxy", metadata.getName())
                .endMetadata()
                .editSpec()
                .editSelector()
                .addToMatchLabels(
                        "consensus-capsule", primary.getSpec().getConsensus().getNode())
                .addToMatchLabels("haproxy", metadata.getName())
                .endSelector()
                .editTemplate()
                .editMetadata()
                .addToLabels(
                        "consensus-capsule", primary.getSpec().getConsensus().getNode())
                .addToLabels("haproxy", metadata.getName())
                .endMetadata()
                .editSpec()
                .editMatchingContainer(c -> "haproxy".equalsIgnoreCase(c.getName()))
                .withEnv(
                        new EnvVar(
                                "HAPROXY_TARGET_IP",
                                context.getSecondaryResources(Service.class).stream()
                                        .filter(svc -> "consensus-node"
                                                .equals(svc.getMetadata()
                                                        .getLabels()
                                                        .get("app")))
                                        .findAny()
                                        .map(Service::getMetadata)
                                        .map(ObjectMeta::getName)
                                        .orElse(null),
                                null),
                        new EnvVar(
                                "HAPROXY_USER", primary.getSpec().getHaproxy().getUser(), null),
                        new EnvVar(
                                "HAPROXY_AUTH", primary.getSpec().getHaproxy().getAuth(), null))
                .endContainer()
                .editMatchingVolume(vb -> "haproxy-cm".equals(vb.getName()))
                .editConfigMap()
                .withName(validName(metadata.getName(), HAProxyConfigMap.NAME_SUFFIX))
                .endConfigMap()
                .endVolume()
                .endSpec()
                .endTemplate()
                .endSpec()
                .build();
    }

    public static class Ready implements Condition<Deployment, HAProxyCapsule> {
        @Override
        public boolean isMet(
                DependentResource<Deployment, HAProxyCapsule> dependentResource,
                HAProxyCapsule primary,
                Context<HAProxyCapsule> context) {
            return context.getSecondaryResources(Service.class).stream().anyMatch(svc -> "consensus-node"
                    .equals(svc.getMetadata().getLabels().get("app")));
        }
    }
}
