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

import static com.hedera.solo.operator.reconciler.common.Constants.SECURITY_CONTEXT_BUILDER;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Sidecars;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.utils.Serialization;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.quarkus.qute.Location;
import io.quarkus.qute.Template;
import io.quarkus.qute.TemplateInstance;
import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.Objects;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=consensus-node")
public class CapsuleDeployment extends CRUDKubernetesDependentResource<Deployment, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-deploy";
    public static final String ACCOUNT_ID_LABEL = "account-id";

    @Location("consensus-capsule/deployment.yaml")
    Template tpl;

    public CapsuleDeployment() {
        super(Deployment.class);
    }

    @Override
    protected Deployment desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        Objects.requireNonNull(primary);
        Objects.requireNonNull(primary.getMetadata());
        Objects.requireNonNull(primary.getMetadata().getName());
        Objects.requireNonNull(primary.getSpec());
        Objects.requireNonNull(primary.getSpec().getAccountId());
        Objects.requireNonNull(context);
        Objects.requireNonNull(context.getClient());

        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final boolean eligibleForDeployment = primary.getSpec().getEligibleForDeployment();

        final Deployment deployment =
                renderDeploymentTemplate(primary, context, primary.getSpec().getAccountId());

        // FIXME standardize labels
        final Map<String, String> labels = Map.of(
                "consensus-capsule",
                metadata.getName(),
                ACCOUNT_ID_LABEL,
                primary.getSpec().getAccountId());

        // return desired deployment
        return deployment
                .edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .addToLabels(labels)
                .endMetadata()
                .editSpec()
                .withReplicas(eligibleForDeployment ? 1 : 0)
                .editSelector()
                .addToMatchLabels(labels)
                .endSelector()
                .editTemplate()
                .editMetadata()
                .addToLabels(labels)
                .endMetadata()
                .endTemplate()
                .endSpec()
                .build();
    }

    private Deployment renderDeploymentTemplate(
            ConsensusCapsule primary, Context<ConsensusCapsule> context, String accountId) {
        Objects.requireNonNull(primary);
        Objects.requireNonNull(primary.getSpec());
        Objects.requireNonNull(primary.getSpec().getSidecars());
        Objects.requireNonNull(context);
        Objects.requireNonNull(context.getClient());
        Objects.requireNonNull(accountId);

        final TemplateInstance tplInstance = tpl.instance();
        final Sidecars sidecars = primary.getSpec().getSidecars();
        final String nodeId = primary.getMetadata().getName();

        // fetch public keys configmap
        final ConfigMap publicKeys = context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("public-keys")
                .get();

        // set template data
        tplInstance.data("nodeId", nodeId);
        tplInstance.data("accountId", accountId);
        tplInstance.data("securityContext", SECURITY_CONTEXT_BUILDER.build());
        tplInstance.data("publicKeyFiles", publicKeys.getData().keySet());
        tplInstance.data("recordStreamUploader", sidecars.getRecordStreamUploader());

        // render deployment template
        final InputStream is = new ByteArrayInputStream(tplInstance.render().getBytes(StandardCharsets.UTF_8));

        // return an instance of Deployment
        return Serialization.unmarshal(is, Deployment.class);
    }
}
