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

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.commonspec.streamuploader.Config;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=consensus-node")
public class CapsuleDeployment extends CRUDKubernetesDependentResource<Deployment, ConsensusCapsule> {

    public static final String TEMPLATE = "deployment.yaml";
    public static final String NAME_SUFFIX = "-deploy";
    public static final String ACCOUNT_ID_LABEL = "account-id";
    public static final SecurityContextBuilder SECURITY_CONTEXT_BUILDER = new SecurityContextBuilder().withRunAsUser(2000L).withRunAsGroup(2000L);
    public static final String RECORD_STREAM_PREFIX = "RECORD_STREAM";

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


        final Map<String, String> clusterConfig = context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("cluster-config")
                .get()
                .getData();

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

            // skip entries like hedera-* (if any)
            if (name.startsWith("hedera")) {
                continue;
            }

            volumeMounts.add(new VolumeMountBuilder()
                    .withName("public-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + name)
                    .withSubPath(name)
                    .build());
        }

        // spotless:off
        final DeploymentBuilder builder = loadYaml(Deployment.class, getClass(), TEMPLATE).edit()
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
                .endContainer()
                .editMatchingInitContainer(c -> "consensus-init".equals(c.getName()))
                .withCommand("/bin/sh", "-c", String.format("""
                        task() {
                          mkdir -p /opt/hgcapp/recordStreams/record%s/sidecar
                          chown -r hedera:hedera /opt/hgcapp/recordStreams
                          return 0
                        }
                        task""", accountId))
                .endInitContainer()
                .endSpec()
                .endTemplate()
                .endSpec();
        // spotless:on

        // prepare sidecars
        this.prepareRecordStreamUploader(primary, clusterConfig, builder);

        return builder.build();
    }

    private void prepareRecordStreamUploader(
            final ConsensusCapsule primary,
            final Map<String, String> clusterConfig,
            final DeploymentBuilder builder) {

        if (!primary.getSpec().getSidecars().getRecordStreamUploader().getEnabled()) {
            return;
        }

        final String accountId = primary.getMetadata().getLabels().get(ACCOUNT_ID_LABEL);
        if (accountId == null || accountId.isEmpty()) {
            throw new RuntimeException("Missing account ID for record-stream-uploader deployment " + primary.getMetadata().getName());
        }

        final String s3Endpoint = clusterConfig.get("s3Endpoint");
        if (s3Endpoint == null || s3Endpoint.isEmpty()) {
            throw new RuntimeException("Missing S3 Endpoint for record-stream-uploader deployment " + primary.getMetadata().getName());
        }

        // setup volume
        final List<VolumeMount> recordStreamVolumeMounts = new ArrayList<>();
        recordStreamVolumeMounts.add(new VolumeMountBuilder()
                .withName("hgcapp-record-streams")
                .withMountPath("/opt/hgcapp/recordStreams")
                .withSubPath(String.format("record%s", accountId))
                .build());


        // setup envVars from sidecar config
        final StreamUploader recordStream = primary.getSpec().getSidecars().getRecordStreamUploader();
        final StreamUploader defaultStreamLoader = loadYaml(StreamUploader.class, getClass(), "defaults/record-stream-uploader.yaml");
        final StreamUploader.ImagePullPolicy imagePullPolicy = recordStream.getImagePullPolicy() != null ?
                recordStream.getImagePullPolicy() : defaultStreamLoader.getImagePullPolicy();
        final List<EnvVar> envVars = this.streamUploaderEnvVars(RECORD_STREAM_PREFIX,
                recordStream.getConfig(), defaultStreamLoader.getConfig());
        envVars.add(new EnvVarBuilder().withName("BUCKET_PATH")
                .withValue(String.format("recordstreams/record%s", accountId)).build());

        builder
                .editSpec()
                .editTemplate()
                .editSpec()
                .addNewContainer()
                .withName(recordStream.getNameOverride())
                .withImage(recordStream.getImage())
                .withImagePullPolicy(imagePullPolicy.toString())
                .withSecurityContext(SECURITY_CONTEXT_BUILDER.build())
                .withCommand(
                        "/usr/bin/env",
                        "python3.7",
                        "/usr/local/bin/mirror.py",
                        "--linux",
                        "--watch-directory",
                        "/opt/hgcapp/recordStreams",
                        "--csv-stats-directory",
                        "/opt/hgcapp/recordStreams/uploader-stats",
                        "--s3-endpoint",
                        s3Endpoint
                )
                .addAllToVolumeMounts(recordStreamVolumeMounts)
                .withEnv(envVars)
                .withEnvFrom(new EnvFromSourceBuilder()
                        .withSecretRef(new SecretEnvSourceBuilder().withName("solo-minio-secrets").build())
                        .build())
                .endContainer()
                .endSpec()
                .endTemplate()
                .endSpec();
    }

    private List<EnvVar> streamUploaderEnvVars(String envPrefix, Config config, Config defaults) {
        final Boolean debug = config.getDebug() != null ? config.getDebug() : defaults.getDebug();
        final Boolean compression = config.getCompression() != null ? config.getCompression() : defaults.getCompression();
        final Boolean sidecar = config.getSidecar() != null ? config.getSidecar() : defaults.getSidecar();
        final Boolean reaperEnabled = config.getReaper().getEnabled() != null ?
                config.getReaper().getEnabled() : defaults.getReaper().getEnabled();
        final Long reaperMinKeep = config.getReaper().getMinKeep() != null ?
                config.getReaper().getMinKeep() : defaults.getReaper().getMinKeep();
        final Long reaperInterval = config.getReaper().getInterval() != null ?
                config.getReaper().getInterval() : defaults.getReaper().getInterval();
        final Long reaperBackoff = config.getReaper().getDefaultBackoff() != null ?
                config.getReaper().getDefaultBackoff() : defaults.getReaper().getDefaultBackoff();
        final Boolean signatureRequired = config.getSignature().getRequire() != null ?
                config.getSignature().getRequire() : defaults.getSignature().getRequire();
        final Boolean signaturePrioritize = config.getSignature().getPrioritize() != null ?
                config.getSignature().getPrioritize() : defaults.getSignature().getPrioritize();
        final String streamFileExtension = config.getStreamFileExtension() != null ?
                config.getStreamFileExtension() : defaults.getStreamFileExtension();
        final String streamSigExtension = config.getStreamSigExtension() != null ?
                config.getStreamSigExtension() : defaults.getStreamSigExtension();
        final String streamExtension = config.getStreamExtension() != null ?
                config.getStreamExtension() : defaults.getStreamExtension();
        final String sigExtension = config.getStreamExtension() != null ?
                config.getSigExtension() : defaults.getSigExtension();
        final String bucketName = config.getBucketName() != null ?
                config.getBucketName() : defaults.getBucketName();
        final Boolean s3Enable = config.getS3Enable() != null ?
                config.getS3Enable() : defaults.getS3Enable();
        final Boolean gcsEnable = config.getGcsEnable() != null ?
                config.getGcsEnable() : defaults.getGcsEnable();


        final List<EnvVar> envVars = new ArrayList<>();
        envVars.add(new EnvVarBuilder().withName("DEBUG").withValue(debug.toString()).build());
        envVars.add(new EnvVarBuilder().withName("REAPER_ENABLE").withValue(reaperEnabled.toString()).build());
        envVars.add(new EnvVarBuilder().withName("REAPER_MIN_KEEP").withValue(reaperMinKeep.toString()).build());
        envVars.add(new EnvVarBuilder().withName("REAPER_INTERVAL").withValue(reaperInterval.toString()).build());
        envVars.add(new EnvVarBuilder().withName("REAPER_DEFAULT_BACKOFF").withValue(reaperBackoff.toString()).build());
        envVars.add(new EnvVarBuilder().withName("STREAM_FILE_EXTENSION").withValue(streamFileExtension).build());
        envVars.add(new EnvVarBuilder().withName("STREAM_SIG_EXTENSION").withValue(streamSigExtension).build());
        envVars.add(new EnvVarBuilder().withName("STREAM_EXTENSION").withValue(streamExtension).build());
        envVars.add(new EnvVarBuilder().withName("SIG_EXTENSION").withValue(sigExtension).build());
        envVars.add(new EnvVarBuilder().withName(String.format("%s_COMPRESSION", envPrefix))
                .withValue(compression.toString()).build());
        envVars.add(new EnvVarBuilder().withName(String.format("%s_SIDECAR", envPrefix))
                .withValue(sidecar.toString()).build());
        envVars.add(new EnvVarBuilder().withName("SIG_REQUIRE")
                .withValue(signatureRequired.toString()).build());
        envVars.add(new EnvVarBuilder().withName("SIG_PRIORITIZE")
                .withValue(signaturePrioritize.toString()).build());
        envVars.add(new EnvVarBuilder().withName("BUCKET_NAME").withValue(bucketName).build());
        envVars.add(new EnvVarBuilder().withName("S3_ENABLE").withValue(s3Enable.toString()).build());
        envVars.add(new EnvVarBuilder().withName("GCS_ENABLE").withValue(gcsEnable.toString()).build());

        return envVars;
    }
}
