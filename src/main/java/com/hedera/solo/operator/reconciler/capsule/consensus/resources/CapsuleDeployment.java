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

import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.SECURITY_CONTEXT_BUILDER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_ACCOUNT_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_BACKUP_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_EVENT_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_PUBLIC_KEY_FILES;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_RECORD_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_SECURITY_CONTEXT;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
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
import io.quarkus.runtime.util.StringUtil;
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
        Objects.requireNonNull(primary.getSpec().getSidecars());
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
        final TemplateInstance tplInstance = tpl.instance();
        final String nodeId = primary.getMetadata().getName();

        // fetch public keys configmap
        final ConfigMap publicKeys = context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("public-keys")
                .get();

        // set template variables
        tplInstance.data(TPL_VAR_NODE_ID, nodeId);
        tplInstance.data(TPL_VAR_ACCOUNT_ID, accountId);
        tplInstance.data(TPL_VAR_SECURITY_CONTEXT, SECURITY_CONTEXT_BUILDER.build());
        tplInstance.data(TPL_VAR_PUBLIC_KEY_FILES, publicKeys.getData().keySet());
        tplInstance.data(TPL_VAR_RECORD_STREAM_UPLOADER, this.prepareRecordStreamUploader(primary));
        tplInstance.data(TPL_VAR_EVENT_STREAM_UPLOADER, this.prepareEventStreamUploader(primary));
        tplInstance.data(TPL_VAR_BACKUP_UPLOADER, this.prepareBackupUploader(primary));

        // return an instance of Deployment
        return Serialization.unmarshal(tplInstance.render(), Deployment.class);
    }

    private StreamUploader prepareRecordStreamUploader(ConsensusCapsule primary) {
        final StreamUploader rsu = this.setStreamUploaderExtensions(
                primary.getSpec().getSidecars().getRecordStreamUploader(),
                RECORD_STREAM_EXTENSION,
                RECORD_STREAM_SIG_EXTENSION);

        if (Objects.nonNull(rsu) && rsu.getConfig().getEnableCompression()) {
            rsu.getConfig().setStreamExtension("%s.gz".formatted(RECORD_STREAM_EXTENSION));
        }

        if (Objects.nonNull(rsu)) {
            rsu.getConfig()
                    .setBucketPath(RECORD_STREAM_BUCKET_PATH_TEMPLATE.formatted(
                            primary.getSpec().getAccountId()));
        }

        return rsu;
    }

    private StreamUploader prepareEventStreamUploader(ConsensusCapsule primary) {
        final StreamUploader esu = this.setStreamUploaderExtensions(
                primary.getSpec().getSidecars().getEventStreamUploader(),
                EVENT_STREAM_EXTENSION,
                EVENT_STREAM_SIG_EXTENSION);

        if (Objects.nonNull(esu)) {
            esu.getConfig()
                    .setBucketPath(EVENT_STREAM_BUCKET_PATH_TEMPLATE.formatted(
                            primary.getSpec().getAccountId()));
        }

        return esu;
    }

    private StreamUploader prepareBackupUploader(ConsensusCapsule primary) {
        final StreamUploader bu = primary.getSpec().getSidecars().getBackupUploader();

        if (Objects.nonNull(bu)) {
            bu.getConfig()
                    .setBucketPath("%s/%s"
                            .formatted(
                                    bu.getConfig().getS3().getUrl(),
                                    bu.getConfig().getBucketName()));
        }

        return bu;
    }

    private StreamUploader setStreamUploaderExtensions(
            final StreamUploader uploader, final String streamExtension, String streamSigExtension) {

        if (Objects.isNull(uploader)) {
            return null;
        }

        Objects.requireNonNull(streamExtension);
        Objects.requireNonNull(streamSigExtension);

        if (StringUtil.isNullOrEmpty(uploader.getConfig().getStreamFileExtension())) {
            uploader.getConfig().setStreamFileExtension(streamExtension);
        }

        if (StringUtil.isNullOrEmpty(uploader.getConfig().getStreamSigExtension())) {
            uploader.getConfig().setStreamSigExtension(streamSigExtension);
        }

        if (StringUtil.isNullOrEmpty(uploader.getConfig().getStreamExtension())) {
            uploader.getConfig().setStreamExtension(streamExtension);
        }

        if (StringUtil.isNullOrEmpty(uploader.getConfig().getSigExtension())) {
            uploader.getConfig().setSigExtension(streamSigExtension);
        }

        return uploader;
    }
}
