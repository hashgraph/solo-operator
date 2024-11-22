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

import com.hedera.solo.operator.model.commonspec.OpenTelemetryCollector;
import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.ConsensusNode;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc.CapsulePvcUtil;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import edu.umd.cs.findbugs.annotations.NonNull;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodBuilder;
import io.fabric8.kubernetes.api.model.PodFluent;
import io.fabric8.kubernetes.api.model.ResourceRequirements;
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
import java.util.function.BiConsumer;
import java.util.function.Function;
import java.util.function.Predicate;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=consensus-node")
public class CapsulePod extends CRUDKubernetesDependentResource<Pod, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-pod";

    public static final String ERR_BOTH_HOSTPATH_PVC_NOT_ALLOWED =
            "a volume cannot have both HostPath and PersistentVolumeClaim specification: '%s'";
    public static final String ERR_DUPLICATE_VOLUME_SPECIFICATION =
            "volume specification is duplicated in persistentVolumeClaims and volumes sections for: %s";
    public static final String OTEL_COLLECTOR = "otel-collector";

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
        PodBuilder podBuilder = Metadata.applyStandardLabels(
                        pod.edit().editMetadata(),
                        validName,
                        namespace,
                        Constants.COMPONENT_CONSENSUS,
                        Constants.COMPONENT_POD,
                        labels)
                .withName(validName)
                .withNamespace(namespace)
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
        copyPodProperties(primary.getSpec().getPodProperties(), podBuilder);

        // force security context if it is not set already
        if (Objects.isNull(podBuilder.build().getSpec().getSecurityContext())) {
            podBuilder
                    .editSpec()
                    .withSecurityContext(SECURITY_CONTEXT_BUILDER.build())
                    .endSpec();
        }
    }

    // FIXME: check property is set before using it
    PodBuilder copyPodProperties(@NonNull final PodProperties props, final PodBuilder builder) {
        Objects.requireNonNull(props);
        PodFluent<PodBuilder>.SpecNested<PodBuilder> spec = builder.editSpec();
        spec.withAffinity(props.getAffinity());
        spec.withNodeSelector(props.getNodeSelector());
        PodProperties.PreemptionPolicy preemptionPolicy = props.getPreemptionPolicy();
        if (Objects.nonNull(preemptionPolicy)) {
            spec.withPreemptionPolicy(preemptionPolicy.getValue());
        }
        spec.withPriority(props.getPriority());
        spec.withPriorityClassName(props.getPriorityClassName());
        spec.withResourceClaims(props.getResourceClaims());

        if (Objects.nonNull(props.getSecurityContext())) {
            spec.withSecurityContext(props.getSecurityContext());
        }

        spec.withServiceAccountName(props.getServiceAccountName());
        spec.withTerminationGracePeriodSeconds(props.getTerminationGracePeriodSeconds());
        spec.withTolerations(props.getTolerations());
        spec.withTopologySpreadConstraints(props.getTopologySpreadConstraints());
        Predicate<String> match = name -> spec.hasMatchingContainer(c -> Objects.equals(name, c.getName()));
        BiConsumer<String, ResourceRequirements> resources =
                (name, rc) -> spec.editMatchingContainer(c -> Objects.equals(name, c.getName()))
                        .withResources(rc)
                        .endContainer();
        List<Function<Containers, ?>> list = List.of(
                Containers::getBackupUploader,
                Containers::getConsensusNode,
                Containers::getEventsStreamUploader,
                Containers::getOtelCollector,
                Containers::getRecordStreamSidecarFilesUploader,
                Containers::getRecordStreamUploader);
        for (Function<Containers, ?> getContainer : list) {
            Containers containers = props.getContainers();
            Object obj = null == containers ? null : getContainer.apply(containers);
            if (obj instanceof ConsensusNode container && match.test(container.getName())) {
                resources.accept(container.getName(), container.getResources());
            } else if (obj instanceof StreamUploader container && match.test(container.getName())) {
                resources.accept(container.getName(), container.getResources());
            } else if (obj instanceof OpenTelemetryCollector container && match.test(OTEL_COLLECTOR)) {
                resources.accept(OTEL_COLLECTOR, container.getResources());
            }
        }
        return spec.endSpec();
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

    protected Optional<PersistentVolumeClaimSpec> getInputPvc(ConsensusCapsule primary, String volNameSuffix) {
        if (!VOLUME_MOUNT_NAMES.containsKey(volNameSuffix)) {
            throw new IllegalArgumentException("invalid volName suffix in: " + volNameSuffix);
        }

        if (primary.getSpec().getPersistentVolumeClaims() == null) {
            return Optional.empty();
        }

        PersistentVolumeClaimSpec inputPvc = null;
        switch (volNameSuffix) {
            case VOLUME_LOGS_NAME_SUFFIX:
                inputPvc = primary.getSpec().getPersistentVolumeClaims().getLogs();
                break;

            case VOLUME_STATES_NAME_SUFFIX:
                inputPvc = primary.getSpec().getPersistentVolumeClaims().getStates();
                break;

            case VOLUME_STATS_NAME_SUFFIX:
                inputPvc = primary.getSpec().getPersistentVolumeClaims().getStats();
                break;

            case VOLUME_UPGRADE_NAME_SUFFIX:
                inputPvc = primary.getSpec().getPersistentVolumeClaims().getUpgrade();
                break;

            case VOLUME_RECORD_STREAM_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims().getStreams() != null) {
                    inputPvc = primary.getSpec()
                            .getPersistentVolumeClaims()
                            .getStreams()
                            .getRecord();
                }
                break;

            case VOLUME_EVENTS_STREAM_NAME_SUFFIX:
                if (primary.getSpec().getPersistentVolumeClaims().getStreams() != null) {
                    inputPvc = primary.getSpec()
                            .getPersistentVolumeClaims()
                            .getStreams()
                            .getEvents();
                }
                break;
            default:
        }

        return Optional.ofNullable(inputPvc);
    }

    protected Optional<Volume> getInputVol(ConsensusCapsule primary, String volNameSuffix) {
        if (!VOLUME_MOUNT_NAMES.containsKey(volNameSuffix)) {
            throw new IllegalArgumentException("invalid volName suffix in: " + volNameSuffix);
        }

        if (primary.getSpec().getPodProperties().getVolumes() == null) {
            return Optional.empty();
        }

        Volume inputVol = null;
        switch (volNameSuffix) {
            case VOLUME_LOGS_NAME_SUFFIX:
                inputVol = primary.getSpec().getPodProperties().getVolumes().getLogs();
                break;

            case VOLUME_STATES_NAME_SUFFIX:
                inputVol = primary.getSpec().getPodProperties().getVolumes().getStates();
                break;

            case VOLUME_STATS_NAME_SUFFIX:
                inputVol = primary.getSpec().getPodProperties().getVolumes().getStats();
                break;

            case VOLUME_UPGRADE_NAME_SUFFIX:
                inputVol = primary.getSpec().getPodProperties().getVolumes().getUpgrade();
                break;

            case VOLUME_RECORD_STREAM_NAME_SUFFIX:
                if (primary.getSpec().getPodProperties().getVolumes().getStreams() != null) {
                    inputVol = primary.getSpec()
                            .getPodProperties()
                            .getVolumes()
                            .getStreams()
                            .getRecord();
                }
                break;

            case VOLUME_EVENTS_STREAM_NAME_SUFFIX:
                if (primary.getSpec().getPodProperties().getVolumes().getStreams() != null) {
                    inputVol = primary.getSpec()
                            .getPodProperties()
                            .getVolumes()
                            .getStreams()
                            .getEvents();
                }
                break;
            default:
        }

        return Optional.ofNullable(inputVol);
    }

    protected Optional<VolumeBuilder> getVolumeBuilder(ConsensusCapsule primary, Volume inputVol, String volName) {
        Objects.requireNonNull(primary);
        VolumeBuilder builder = new VolumeBuilder().withName(volName);

        if (inputVol == null) {
            return Optional.empty();
        }

        // a volume cannot be both hostpath and PVC
        if (inputVol.getHostPath() != null && inputVol.getPersistentVolumeClaim() != null) {
            throw new IllegalArgumentException(ERR_BOTH_HOSTPATH_PVC_NOT_ALLOWED.formatted(volName));
        }

        // return the volume builder based on the input volume
        if (inputVol.getHostPath() != null) {
            builder.withHostPath(inputVol.getHostPath());
        } else if (inputVol.getPersistentVolumeClaim() != null) {
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
        VOLUME_MOUNT_NAMES.forEach((volNameSuffix, volName) -> {
            final Optional<PersistentVolumeClaimSpec> inputPvc = getInputPvc(primary, volNameSuffix);
            final Optional<Volume> inputVol = getInputVol(primary, volNameSuffix);

            if (inputPvc.isPresent() && inputVol.isPresent()) {
                throw new IllegalArgumentException(ERR_DUPLICATE_VOLUME_SPECIFICATION.formatted(volNameSuffix));
            }

            // if PVC is present, it takes priority over the volumes settings for a volume.
            if (inputPvc.isPresent()) {
                VolumeBuilder builder = new VolumeBuilder()
                        .withName(volName)
                        .withPersistentVolumeClaim(new PersistentVolumeClaimVolumeSourceBuilder()
                                .withClaimName(CapsulePvcUtil.validPvcName(primary, volNameSuffix))
                                .build());
                volumes.add(builder.build());
            } else {
                if (inputVol.isPresent()) {
                    final Optional<VolumeBuilder> vb = getVolumeBuilder(primary, inputVol.get(), volName);
                    vb.ifPresent(volumeBuilder -> volumes.add(volumeBuilder.build()));
                }
            }
        });

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
