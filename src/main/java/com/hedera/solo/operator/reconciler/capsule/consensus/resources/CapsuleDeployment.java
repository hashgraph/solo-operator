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

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.ArrayList;
import java.util.List;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=consensus-node")
public class CapsuleDeployment extends CRUDKubernetesDependentResource<Deployment, ConsensusCapsule> {

    public static final String TEMPLATE = "deployment.yaml";
    public static final String NAME_SUFFIX = "-deploy";
    public static final String ACCOUNT_ID_LABEL = "account-id";

    public CapsuleDeployment() {
        super(Deployment.class);
    }

    @Override
    protected Deployment desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);

        final String accountId = metadata.getLabels().get(ACCOUNT_ID_LABEL);
        if (accountId == null || accountId.isEmpty()) {
            throw new RuntimeException("Missing account ID for deployment " + primary.getMetadata().getName());
        }

        final boolean eligibleForDeployment =
                primary.getSpec() != null && primary.getSpec().getEligibleForDeployment();

        final List<VolumeMount> volumeMounts = new ArrayList<>();
        for (String name : List.of("a", "s")) {
            String filename = name + "-private-" + metadata.getName() + ".pem";
            volumeMounts.add(new VolumeMountBuilder()
                    .withName("private-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + filename)
                    .withSubPath(filename)
                    .build());
        }

        for (String name : context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("public-keys")
                .get()
                .getData()
                .keySet()) {

            // skip entries like hedera-*
            if (name.startsWith("hedera")) {
                continue;
            }

            volumeMounts.add(new VolumeMountBuilder()
                    .withName("public-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + name)
                    .withSubPath(name)
                    .build());
        }

        final Probe startupProbe = new ProbeBuilder()
                .editExec()
                .withCommand("/bin/sh", "-c", String.format("""
                        task() {
                          mkdir -p /opt/hgcapp/recordStreams/record%s/sidecar
                          chown -r hedera:hedera /opt/hgcapp/recordStreams
                          # enable below curl command and remove the return statement once the otelCollector sidecar is enabled
                          #curl %s.consensus.%s.svc.cluster.local:13133
                          return 0
                        }
                        task""", accountId, metadata.getName(), metadata.getNamespace()))
                .endExec()
                .withFailureThreshold(30)
                .withPeriodSeconds(10)
                .withTimeoutSeconds(5)
                .build();

        // spotless:off
        return loadYaml(Deployment.class, getClass(), TEMPLATE).edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .addToLabels("consensus-capsule", metadata.getName())
                .endMetadata()
                .editSpec()
                .withReplicas(eligibleForDeployment ? 1 : 0)
                .editSelector()
                .addToMatchLabels("consensus-capsule", metadata.getName())
                .endSelector()
                .editTemplate()
                .editMetadata()
                .addToLabels("name", metadata.getName())
                .addToLabels("consensus-capsule", metadata.getName())
                .addToLabels(ACCOUNT_ID_LABEL, accountId)
                .endMetadata()
                .editSpec()
                .withHostname(metadata.getName())
                .editMatchingVolume(v -> "private-keys".equals(v.getName()))
                .editSecret()
                .withSecretName(validName(metadata.getName(), "-keys"))
                .endSecret()
                .endVolume()
                .editMatchingContainer(c -> "consensus-node".equals(c.getName()))
                .addAllToVolumeMounts(volumeMounts)
                .withStartupProbe(startupProbe)
                .endContainer()
                .endSpec()
                .endTemplate()
                .endSpec()
                .build();
        // spotless:on
    }
}
