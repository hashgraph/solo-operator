// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Constants.DEFAULT_FILE_MODE;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_ACCOUNT_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.SECURITY_CONTEXT_BUILDER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_ACCOUNT_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_BACKUP_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_DEFAULT_MODE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_EVENT_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NAMESPACE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_NAME;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_NAMES;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_PUBLIC_KEY_FILES;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_RECORD_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_LOGS_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_MOUNT_NAMES;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_STATES_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_STATS_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_UPGRADE_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.ConsensusNode;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodBuilder;
import io.fabric8.kubernetes.api.model.PodSecurityContext;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.fabric8.kubernetes.client.utils.Serialization;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;
import io.quarkus.qute.Location;
import io.quarkus.qute.Template;
import io.quarkus.qute.TemplateInstance;
import io.quarkus.runtime.util.StringUtil;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=consensus-node")
public class CapsulePod extends CRUDKubernetesDependentResource<Pod, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-pod";

    public static final String ERR_PERSISTENT_VOL_NOT_SPECIFIED =
            "persistent volume claims not specified for volume: %s";
    public static final String ERR_BOTH_HOSTPATH_PVC_NOT_ALLOWED =
            "both HostPath and PersistentVolumeClaim is not allowed for the same volume: '%s'";
    public static final String ERR_INVALID_VOL_TYPE =
            "volume must be either a host path or a persistent volume claim or emptyDir for: %s";
    public static final String ERR_UNSUPPORTED_VOLUME_TYPES =
            "supported volumes are emptyDir, hostPath, and PersistentVolumeClaim, none provided.";

    @Location("consensus-capsule/pod.yaml")
    Template tpl;

    public CapsulePod() {
        super(Pod.class);
    }

    /**
     * Validate the input
     *
     * @param primary ConsensusCapsule instance
     * @param context An instance of context
     * @throws NullPointerException if required inputs are missing
     * @throws IllegalArgumentException if invalid input is specified
     */
    private void validateInput(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        Objects.requireNonNull(primary);
        Objects.requireNonNull(primary.getMetadata());
        Objects.requireNonNull(primary.getMetadata().getName());
        Objects.requireNonNull(primary.getMetadata().getNamespace());
        Objects.requireNonNull(primary.getSpec());
        Objects.requireNonNull(primary.getSpec().getOrbit());
        Objects.requireNonNull(primary.getSpec().getAccountId());
        Objects.requireNonNull(primary.getSpec().getPodProperties());
        Objects.requireNonNull(primary.getSpec().getPodProperties().getContainers());
        Objects.requireNonNull(
                primary.getSpec().getPodProperties().getContainers().getConsensusNode());
        Objects.requireNonNull(context);
        Objects.requireNonNull(context.getClient());

        // namespace must match with the orbit name
        final String orbit = primary.getSpec().getOrbit();
        if (!orbit.equals(primary.getMetadata().getNamespace())) {
            throw new IllegalArgumentException("namespace must match with the orbit name '%s', found '%s'"
                    .formatted(orbit, primary.getMetadata().getNamespace()));
        }
    }

    @Override
    protected Pod desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        this.validateInput(primary, context);

        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = primary.getMetadata().getNamespace();

        final Pod pod = renderDeploymentTemplate(primary, context);

        // FIXME standardize labels
        final Map<String, String> labels = Map.of(
                "consensus-capsule",
                metadata.getName(),
                LABEL_ACCOUNT_ID,
                primary.getSpec().getAccountId(),
                LABEL_NODE_ID,
                primary.getSpec().getNodeId().toString());

        // return desired pod
        PodBuilder podBuilder = pod.edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(namespace)
                .addToLabels(labels)
                .endMetadata();

        this.setDesiredPodSpec(primary, podBuilder);

        // set volumes
        final List<Volume> desiredVolumes = getDesiredPodVolumes(primary);
        for (final Volume v : desiredVolumes) {
            podBuilder
                    .editSpec()
                    .removeMatchingFromVolumes((s) -> Objects.equals(v.getName(), s.getName()))
                    .addToVolumes(v)
                    .endSpec();
        }

        return podBuilder.build();
    }

    private void setDesiredPodSpec(ConsensusCapsule primary, PodBuilder podBuilder) {
        PodSecurityContext securityContext =
                primary.getSpec().getPodProperties().getSecurityContext();
        if (securityContext == null) {
            securityContext = SECURITY_CONTEXT_BUILDER.build();
        }

        // TODO add other pod spec and associated test #263

        podBuilder.editSpec().withSecurityContext(securityContext).endSpec();
    }

    private TemplateInstance getTemplateWithDefaults(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        final Long nodeId = primary.getSpec().getNodeId();
        final String nodeName = Metadata.prepareNodeName(primary);
        final String accountId = primary.getSpec().getAccountId();
        final String namespace = primary.getMetadata().getNamespace();
        final List<String> nodeNames = getNodeNames(context, namespace);

        final TemplateInstance templateInstance = tpl.instance();

        // set defaults
        templateInstance.data(TPL_VAR_NAMESPACE, namespace);
        templateInstance.data(TPL_VAR_NODE_ID, nodeId);
        templateInstance.data(TPL_VAR_NODE_NAME, nodeName);
        templateInstance.data(TPL_VAR_NODE_NAMES, nodeNames);
        templateInstance.data(TPL_VAR_ACCOUNT_ID, accountId);
        templateInstance.data(TPL_VAR_DEFAULT_MODE, DEFAULT_FILE_MODE);

        return templateInstance;
    }

    protected Map<String, Optional<StreamUploader>> prepareStreamUploaderContainers(ConsensusCapsule primary) {
        final Optional<StreamUploader> rsu = this.prepareRecordStreamUploader(primary);
        final Optional<StreamUploader> esu = this.prepareEventStreamUploader(primary);
        final Optional<StreamUploader> bu = this.prepareBackupUploader(primary);
        final Optional<StreamUploader> rssu = this.prepareRecordStreamSidecarFilesUploader(primary);

        return Map.of(
                TPL_VAR_RECORD_STREAM_UPLOADER, rsu,
                TPL_VAR_EVENT_STREAM_UPLOADER, esu,
                TPL_VAR_BACKUP_UPLOADER, bu,
                TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER, rssu);
    }

    protected Pod renderDeploymentTemplate(ConsensusCapsule primary, Context<ConsensusCapsule> context) {

        // fetch public keys configmap
        final ConfigMap publicKeys = context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("public-keys")
                .get();

        final ConsensusNode cn =
                primary.getSpec().getPodProperties().getContainers().getConsensusNode();
        final Map<String, Optional<StreamUploader>> uploaderMap = this.prepareStreamUploaderContainers(primary);

        // set template variables
        TemplateInstance tplInstance = getTemplateWithDefaults(primary, context);
        tplInstance.data(TPL_VAR_PUBLIC_KEY_FILES, publicKeys.getData().keySet());
        tplInstance.data(TPL_VAR_CONSENSUS_NODE, cn);
        uploaderMap.forEach((k, v) -> tplInstance.data(k, v.orElse(null)));

        // return an instance of Pod
        return Serialization.unmarshal(tplInstance.render(), Pod.class);
    }

    private Optional<StreamUploader> prepareRecordStreamUploader(ConsensusCapsule primary) {
        final Optional<StreamUploader> rsu = this.setStreamFileExtensions(
                primary.getSpec().getPodProperties().getContainers().getRecordStreamUploader(),
                RECORD_STREAM_EXTENSION,
                RECORD_STREAM_SIG_EXTENSION);

        rsu.ifPresent(uploader -> {
            if (uploader.getConfig().getEnableCompression()) {
                uploader.getConfig().setStreamExtension("%s.gz".formatted(RECORD_STREAM_EXTENSION));
            }

            uploader.getConfig()
                    .setBucketPath(RECORD_STREAM_BUCKET_PATH_TEMPLATE.formatted(
                            primary.getSpec().getAccountId()));
        });

        return rsu;
    }

    private Optional<StreamUploader> prepareRecordStreamSidecarFilesUploader(ConsensusCapsule primary) {
        final Optional<StreamUploader> rssu = this.setStreamFileExtensions(
                primary.getSpec().getPodProperties().getContainers().getRecordStreamSidecarFilesUploader(),
                RECORD_STREAM_EXTENSION,
                RECORD_STREAM_SIG_EXTENSION);

        rssu.ifPresent(uploader -> {
            if (uploader.getConfig().getEnableCompression()) {
                uploader.getConfig().setStreamExtension("%s.gz".formatted(RECORD_STREAM_EXTENSION));
            }

            uploader.getConfig()
                    .setBucketPath(RECORD_STREAM_BUCKET_PATH_TEMPLATE.formatted(
                            primary.getSpec().getAccountId()));

            // force it to be false as we don't need signature for sidecar files
            uploader.getConfig().getSignature().setPrioritize(false);
            uploader.getConfig().getSignature().setRequire(false);
        });

        return rssu;
    }

    private Optional<StreamUploader> prepareEventStreamUploader(ConsensusCapsule primary) {
        final Optional<StreamUploader> esu = this.setStreamFileExtensions(
                primary.getSpec().getPodProperties().getContainers().getEventsStreamUploader(),
                EVENT_STREAM_EXTENSION,
                EVENT_STREAM_SIG_EXTENSION);

        esu.ifPresent(uploader -> uploader.getConfig()
                .setBucketPath(EVENT_STREAM_BUCKET_PATH_TEMPLATE.formatted(
                        primary.getSpec().getAccountId())));

        return esu;
    }

    private Optional<StreamUploader> prepareBackupUploader(ConsensusCapsule primary) {
        final StreamUploader bu =
                primary.getSpec().getPodProperties().getContainers().getBackupUploader();

        if (Objects.isNull(bu) || !bu.getEnabled()) {
            return Optional.empty();
        }

        bu.getConfig()
                .setBucketPath("%s/%s"
                        .formatted(
                                bu.getConfig().getS3().getUrl(), bu.getConfig().getBucketName()));

        return Optional.of(bu);
    }

    protected Optional<StreamUploader> setStreamFileExtensions(
            final StreamUploader uploader, final String streamExtension, String streamSigExtension) {

        if (Objects.isNull(uploader) || !uploader.getEnabled()) {
            return Optional.empty();
        }

        Objects.requireNonNull(streamExtension);
        Objects.requireNonNull(streamSigExtension);

        // set file extensions
        if (StringUtil.isNullOrEmpty(uploader.getConfig().getStreamFileExtension())) {
            uploader.getConfig().setStreamFileExtension(streamExtension);
        }
        if (StringUtil.isNullOrEmpty(uploader.getConfig().getStreamExtension())) {
            uploader.getConfig().setStreamExtension(streamExtension);
        }

        // set sig extensions
        if (StringUtil.isNullOrEmpty(uploader.getConfig().getStreamSigExtension())) {
            uploader.getConfig().setStreamSigExtension(streamSigExtension);
        }
        if (StringUtil.isNullOrEmpty(uploader.getConfig().getSigExtension())) {
            uploader.getConfig().setSigExtension(streamSigExtension);
        }

        return Optional.of(uploader);
    }

    private static List<String> getNodeNames(Context<ConsensusCapsule> context, String namespace) {
        final List<String> nodeIds = new ArrayList<>();

        final ConfigMap clusterConfig = context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(namespace)
                .withName("cluster-config")
                .get();

        // extract the node-ids from config.txt
        final String configTxt = clusterConfig.getData().get("config.txt");
        for (String line : configTxt.split("\n")) {
            if (line.startsWith("address")) {
                final String[] parts = line.split(",");
                if (parts.length != 10) {
                    throw new IllegalArgumentException("Invalid address line in config.txt: " + line);
                }

                // pick the node nickname/alias at index 2
                // FIXME for DAB Phase-3, we may need to revisit this
                nodeIds.add(parts[2].trim());
            }
        }

        return nodeIds.stream().sorted().toList();
    }

    protected void validatePvcIsSpecified(ConsensusCapsule primary, String volNameSuffix)
            throws IllegalArgumentException {
        final String volName = VOLUME_MOUNT_NAMES.get(volNameSuffix);
        if (Objects.isNull(volName)) {
            throw new IllegalArgumentException("invalid volName suffix in: " + volNameSuffix);
        }

        switch (volNameSuffix) {
            case VOLUME_LOGS_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims() == null
                        || primary.getSpec().getPersistentVolumeClaims().getLogs() == null) {
                    throw new IllegalArgumentException(ERR_PERSISTENT_VOL_NOT_SPECIFIED.formatted(volName));
                }
                break;

            case VOLUME_STATES_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims() == null
                        || primary.getSpec().getPersistentVolumeClaims().getStates() == null) {
                    throw new IllegalArgumentException(ERR_PERSISTENT_VOL_NOT_SPECIFIED.formatted(volName));
                }
                break;

            case VOLUME_STATS_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims() == null
                        || primary.getSpec().getPersistentVolumeClaims().getStats() == null) {
                    throw new IllegalArgumentException(ERR_PERSISTENT_VOL_NOT_SPECIFIED.formatted(volName));
                }
                break;

            case VOLUME_UPGRADE_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims() == null
                        || primary.getSpec().getPersistentVolumeClaims().getUpgrade() == null) {
                    throw new IllegalArgumentException(ERR_PERSISTENT_VOL_NOT_SPECIFIED.formatted(volName));
                }
                break;

            case VOLUME_RECORD_STREAM_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims() == null
                        || primary.getSpec().getPersistentVolumeClaims().getStreams() == null
                        || primary.getSpec()
                                        .getPersistentVolumeClaims()
                                        .getStreams()
                                        .getRecord()
                                == null) {
                    throw new IllegalArgumentException(ERR_PERSISTENT_VOL_NOT_SPECIFIED.formatted(volName));
                }
                break;

            case VOLUME_EVENTS_STREAM_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims() == null
                        || primary.getSpec().getPersistentVolumeClaims().getStreams() == null
                        || primary.getSpec()
                                        .getPersistentVolumeClaims()
                                        .getStreams()
                                        .getEvents()
                                == null) {
                    throw new IllegalArgumentException(ERR_PERSISTENT_VOL_NOT_SPECIFIED.formatted(volName));
                }
                break;
            default:
        }
    }

    protected Optional<VolumeBuilder> getVolumeBuilder(
            ConsensusCapsule primary, Volume inputVol, String volNameSuffix) {
        Objects.requireNonNull(primary);
        Objects.requireNonNull(inputVol);
        Objects.requireNonNull(volNameSuffix);

        final String volName = VOLUME_MOUNT_NAMES.get(volNameSuffix);
        if (Objects.isNull(volName)) {
            throw new IllegalArgumentException("invalid volName suffix: " + volNameSuffix);
        }

        if (inputVol.getHostPath() != null && inputVol.getPersistentVolumeClaim() != null) {
            throw new IllegalArgumentException(ERR_BOTH_HOSTPATH_PVC_NOT_ALLOWED.formatted(volName));
        }

        VolumeBuilder builder = new VolumeBuilder().withName(volName);
        if (inputVol.getHostPath() != null) {
            builder.withHostPath(inputVol.getHostPath());
        } else if (inputVol.getPersistentVolumeClaim() != null) {
            validatePvcIsSpecified(primary, volNameSuffix);
            builder.withPersistentVolumeClaim(inputVol.getPersistentVolumeClaim());
        } else if (inputVol.getEmptyDir() != null) {
            builder.withEmptyDir(inputVol.getEmptyDir());
        } else {
            return Optional.empty(); // for unsupported types, we return null;
        }

        return Optional.of(builder);
    }

    protected List<Volume> getDesiredPodVolumes(ConsensusCapsule primary) {
        final List<Volume> volumes = new ArrayList<>();
        if (primary.getSpec().getPodProperties().getVolumes().getLogs() != null) {
            final Volume inputVol =
                    primary.getSpec().getPodProperties().getVolumes().getLogs();
            final Optional<VolumeBuilder> vb = getVolumeBuilder(primary, inputVol, VOLUME_LOGS_NAME_SUFFIX);
            vb.ifPresent(volumeBuilder -> volumes.add(volumeBuilder.build()));
        }

        if (primary.getSpec().getPodProperties().getVolumes().getStats() != null) {
            final Volume inputVol =
                    primary.getSpec().getPodProperties().getVolumes().getStats();
            final Optional<VolumeBuilder> vb = getVolumeBuilder(primary, inputVol, VOLUME_STATS_NAME_SUFFIX);
            vb.ifPresent(volumeBuilder -> volumes.add(volumeBuilder.build()));
        }

        if (primary.getSpec().getPodProperties().getVolumes().getStates() != null) {
            final Volume inputVol =
                    primary.getSpec().getPodProperties().getVolumes().getStates();
            final Optional<VolumeBuilder> vb = getVolumeBuilder(primary, inputVol, VOLUME_STATES_NAME_SUFFIX);
            vb.ifPresent(volumeBuilder -> volumes.add(volumeBuilder.build()));
        }

        if (primary.getSpec().getPodProperties().getVolumes().getUpgrade() != null) {
            final Volume inputVol =
                    primary.getSpec().getPodProperties().getVolumes().getUpgrade();
            final Optional<VolumeBuilder> vb = getVolumeBuilder(primary, inputVol, VOLUME_UPGRADE_NAME_SUFFIX);
            vb.ifPresent(volumeBuilder -> volumes.add(volumeBuilder.build()));
        }

        if (primary.getSpec().getPodProperties().getVolumes().getStreams() != null) {
            if (primary.getSpec().getPodProperties().getVolumes().getStreams().getRecord() != null) {
                final Volume inputVol = primary.getSpec()
                        .getPodProperties()
                        .getVolumes()
                        .getStreams()
                        .getRecord();
                final Optional<VolumeBuilder> vb =
                        getVolumeBuilder(primary, inputVol, VOLUME_RECORD_STREAM_NAME_SUFFIX);
                vb.ifPresent(volumeBuilder -> volumes.add(volumeBuilder.build()));
            }

            if (primary.getSpec().getPodProperties().getVolumes().getStreams().getEvents() != null) {
                final Volume inputVol = primary.getSpec()
                        .getPodProperties()
                        .getVolumes()
                        .getStreams()
                        .getEvents();
                final Optional<VolumeBuilder> vb =
                        getVolumeBuilder(primary, inputVol, VOLUME_EVENTS_STREAM_NAME_SUFFIX);
                vb.ifPresent(volumeBuilder -> volumes.add(volumeBuilder.build()));
            }
        }

        return volumes;
    }

    public static class Ready implements Condition<Pod, ConsensusCapsule> {
        @Override
        public boolean isMet(
                DependentResource<Pod, ConsensusCapsule> dependentResource,
                ConsensusCapsule primary,
                Context<ConsensusCapsule> context) {
            return primary.getSpec().getEligibleForDeployment();
        }
    }
}
