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
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
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
    public static final SecurityContextBuilder SECURITY_CONTEXT_BUILDER =
            new SecurityContextBuilder().withRunAsUser(2000L).withRunAsGroup(2000L);
    public static final String RECORD_STREAM_PREFIX = "RECORD_STREAM";
    public static final String RECORD_STREAM_TEMPLATE = "sidecars/record-stream-uploader.yaml";

    public CapsuleDeployment() {
        super(Deployment.class);
    }

    @Override
    protected Deployment desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String accountId = metadata.getLabels().get(ACCOUNT_ID_LABEL);
        if (accountId == null || accountId.isEmpty()) {
            throw new RuntimeException(
                    "Missing account ID for deployment " + primary.getMetadata().getName());
        }

        final List<VolumeMount> volumeMounts = prepareVolumeMountsMapping(primary, context, metadata);
        final boolean eligibleForDeployment =
                primary.getSpec() != null && primary.getSpec().getEligibleForDeployment();

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
        this.prepareRecordStreamUploader(primary, builder);

        return builder.build();
    }

    private static List<VolumeMount> prepareVolumeMountsMapping(
            ConsensusCapsule primary, Context<ConsensusCapsule> context, ObjectMeta metadata) {
        final List<VolumeMount> volumeMounts = new ArrayList<>();
        for (String nameTemplate : Constants.NODE_KEYS_FILENAMES_TEMPLATE) {
            String subPath = String.format(nameTemplate, metadata.getName());

            String mounthPath = String.format("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/%s", subPath);
            if (nameTemplate.equals("hedera-%s.crt")) {
                mounthPath = "/opt/hgcapp/services-hedera/HapiApp2.0/hedera.crt";
            } else if (nameTemplate.equals("hedera-%s.key")) {
                mounthPath = "/opt/hgcapp/services-hedera/HapiApp2.0/hedera.key";
            }

            volumeMounts.add(new VolumeMountBuilder()
                    .withName("private-keys")
                    .withMountPath(mounthPath)
                    .withSubPath(subPath)
                    .build());
        }

        for (String name : context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("public-keys")
                .get()
                .getData()
                .keySet()) {

            volumeMounts.add(new VolumeMountBuilder()
                    .withName("public-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + name)
                    .withSubPath(name)
                    .build());
        }
        return volumeMounts;
    }

    private void prepareRecordStreamUploader(final ConsensusCapsule primary, final DeploymentBuilder builder) {

        if (primary.getSpec().getSidecars() == null
                || primary.getSpec().getSidecars().getRecordStreamUploader() == null
                || !primary.getSpec().getSidecars().getRecordStreamUploader().getEnabled()) {
            return;
        }

        final StreamUploader recordStream = primary.getSpec().getSidecars().getRecordStreamUploader();

        // validate record-stream-uploader sidecar spec
        if (recordStream.getConfig() == null) {
            throw new RuntimeException("Missing record stream uploader config");
        }

        if (recordStream.getConfig().getS3() == null) {
            throw new RuntimeException("Missing record stream uploader S3 config");
        }

        if (recordStream.getConfig().getGcs() == null) {
            throw new RuntimeException("Missing record stream uploader GCS config");
        }

        final StreamUploader defaultStreamLoader = loadYaml(StreamUploader.class, getClass(), RECORD_STREAM_TEMPLATE);

        final String accountId = primary.getMetadata().getLabels().get(ACCOUNT_ID_LABEL);
        if (accountId == null || accountId.isEmpty()) {
            throw new RuntimeException("Missing account ID for record-stream-uploader deployment "
                    + primary.getMetadata().getName());
        }

        final String s3Endpoint = recordStream.getConfig().getS3() != null
                ? recordStream.getConfig().getS3().getUrl()
                : defaultStreamLoader.getConfig().getS3().getUrl();
        final String gcsEndpoint = recordStream.getConfig().getGcs() != null
                ? recordStream.getConfig().getGcs().getUrl()
                : defaultStreamLoader.getConfig().getGcs().getUrl();

        if ((s3Endpoint == null || s3Endpoint.isEmpty()) && (gcsEndpoint == null || gcsEndpoint.isEmpty())) {
            throw new RuntimeException("Either s3-endpoint gcs-endpoint URL is required");
        }

        // setup volume
        final List<VolumeMount> recordStreamVolumeMounts = new ArrayList<>();
        recordStreamVolumeMounts.add(new VolumeMountBuilder()
                .withName("hgcapp-record-streams")
                .withMountPath("/opt/hgcapp/recordStreams")
                .withSubPath(String.format("record%s", accountId))
                .build());

        // setup envVars from sidecar config
        final StreamUploader.ImagePullPolicy imagePullPolicy = recordStream.getImagePullPolicy() != null
                ? recordStream.getImagePullPolicy()
                : defaultStreamLoader.getImagePullPolicy();
        final List<EnvVar> envVars = this.streamUploaderEnvVars(
                recordStream.getConfig(), defaultStreamLoader.getConfig(), RECORD_STREAM_PREFIX);
        envVars.add(new EnvVarBuilder()
                .withName("BUCKET_PATH")
                .withValue(String.format("recordstreams/record%s", accountId))
                .build());
        final String name = recordStream.getNameOverride() != null
                ? recordStream.getNameOverride()
                : defaultStreamLoader.getNameOverride();
        final String image = recordStream.getImage() != null ? recordStream.getImage() : defaultStreamLoader.getImage();
        final ArrayList<String> uploaderCommand = getUploadCmd(s3Endpoint, gcsEndpoint);

        builder.editSpec()
                .editTemplate()
                .editSpec()
                .addNewContainer()
                .withName(name)
                .withImage(image)
                .withImagePullPolicy(imagePullPolicy.toString())
                .withSecurityContext(SECURITY_CONTEXT_BUILDER.build())
                .withCommand(uploaderCommand)
                .addAllToVolumeMounts(recordStreamVolumeMounts)
                .withEnv(envVars)
                .withEnvFrom(new EnvFromSourceBuilder()
                        .withSecretRef(new SecretEnvSourceBuilder()
                                .withName("uploader-secrets")
                                .build())
                        .build())
                .endContainer()
                .endSpec()
                .endTemplate()
                .endSpec();
    }

    private static ArrayList<String> getUploadCmd(String s3Endpoint, String gcsEndpoint) {
        final ArrayList<String> uploaderCommand = new ArrayList<>(List.of(
                "/usr/bin/env",
                "python3.7",
                "/usr/local/bin/mirror.py",
                "--linux",
                "--debug",
                "--watch-directory",
                "/opt/hgcapp/recordStreams",
                "--csv-stats-directory",
                "/opt/hgcapp/recordStreams/uploader-stats"));

        if (s3Endpoint != null) {
            uploaderCommand.add("--s3-endpoint");
            uploaderCommand.add(s3Endpoint);
        }

        if (gcsEndpoint != null) {
            uploaderCommand.add("--gcs-endpoint");
            uploaderCommand.add(gcsEndpoint);
        }

        return uploaderCommand;
    }

    private List<EnvVar> streamUploaderEnvVars(Config config, Config defaults, String envPrefix) {
        final Boolean debug = config.getDebug() != null ? config.getDebug() : defaults.getDebug();
        final Boolean compression =
                config.getCompression() != null ? config.getCompression() : defaults.getCompression();
        final Boolean sidecar = config.getSidecar() != null ? config.getSidecar() : defaults.getSidecar();
        final Boolean reaperEnabled =
                config.getReaper() != null && config.getReaper().getEnabled() != null
                        ? config.getReaper().getEnabled()
                        : defaults.getReaper().getEnabled();
        final Long reaperMinKeep =
                config.getReaper() != null && config.getReaper().getMinKeep() != null
                        ? config.getReaper().getMinKeep()
                        : defaults.getReaper().getMinKeep();
        final Long reaperInterval =
                config.getReaper() != null && config.getReaper().getInterval() != null
                        ? config.getReaper().getInterval()
                        : defaults.getReaper().getInterval();
        final Long reaperBackoff =
                config.getReaper() != null && config.getReaper().getDefaultBackoff() != null
                        ? config.getReaper().getDefaultBackoff()
                        : defaults.getReaper().getDefaultBackoff();
        final Boolean signatureRequired =
                config.getSignature() != null && config.getSignature().getRequire() != null
                        ? config.getSignature().getRequire()
                        : defaults.getSignature().getRequire();
        final Boolean signaturePrioritize =
                config.getSignature() != null && config.getSignature().getPrioritize() != null
                        ? config.getSignature().getPrioritize()
                        : defaults.getSignature().getPrioritize();
        final String streamFileExtension = config.getStreamFileExtension() != null
                ? config.getStreamFileExtension()
                : defaults.getStreamFileExtension();
        final String streamSigExtension = config.getStreamSigExtension() != null
                ? config.getStreamSigExtension()
                : defaults.getStreamSigExtension();
        final String streamExtension =
                config.getStreamExtension() != null ? config.getStreamExtension() : defaults.getStreamExtension();
        final String sigExtension =
                config.getStreamExtension() != null ? config.getSigExtension() : defaults.getSigExtension();
        final String bucketName = config.getBucketName() != null ? config.getBucketName() : defaults.getBucketName();
        final Boolean s3Enable = config.getS3().getEnabled() != null
                ? config.getS3().getEnabled()
                : defaults.getS3().getEnabled();
        final Boolean gcsEnable = config.getGcs().getEnabled() != null
                ? config.getGcs().getEnabled()
                : defaults.getGcs().getEnabled();

        final String s3AccessKey = config.getS3().getAccessKey() != null
                ? config.getS3().getAccessKey()
                : defaults.getS3().getAccessKey();
        final String gcsAccessKey = config.getGcs().getAccessKey() != null
                ? config.getGcs().getAccessKey()
                : defaults.getGcs().getAccessKey();

        final List<EnvVar> envVars = new ArrayList<>();
        envVars.add(new EnvVarBuilder()
                .withName("DEBUG")
                .withValue(debug.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("REAPER_ENABLE")
                .withValue(reaperEnabled.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("REAPER_MIN_KEEP")
                .withValue(reaperMinKeep.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("REAPER_INTERVAL")
                .withValue(reaperInterval.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("REAPER_DEFAULT_BACKOFF")
                .withValue(reaperBackoff.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("STREAM_FILE_EXTENSION")
                .withValue(streamFileExtension)
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("STREAM_SIG_EXTENSION")
                .withValue(streamSigExtension)
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("STREAM_EXTENSION")
                .withValue(streamExtension)
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("SIG_EXTENSION")
                .withValue(sigExtension)
                .build());
        envVars.add(new EnvVarBuilder()
                .withName(String.format("%s_COMPRESSION", envPrefix))
                .withValue(compression.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName(String.format("%s_SIDECAR", envPrefix))
                .withValue(sidecar.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("SIG_REQUIRE")
                .withValue(signatureRequired.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("SIG_PRIORITIZE")
                .withValue(signaturePrioritize.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("BUCKET_NAME")
                .withValue(bucketName)
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("S3_ENABLE")
                .withValue(s3Enable.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("S3_ACCESS_KEY")
                .withValue(s3AccessKey)
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("S3_ACCESS_KEY")
                .withValue(s3AccessKey)
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("GCS_ENABLE")
                .withValue(gcsEnable.toString())
                .build());
        envVars.add(new EnvVarBuilder()
                .withName("GCS_ACCESS_KEY")
                .withValue(gcsAccessKey)
                .build());

        return envVars;
    }
}
