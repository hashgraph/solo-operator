// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleUtils.secretsWithDefaultKeyNames;
import static com.hedera.solo.operator.reconciler.common.Constants.DEFAULT_FILE_MODE;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.SECURITY_CONTEXT_BUILDER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_ACCOUNT_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_BACKUP_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_CONFIG;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_DEFAULT_MODE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_EVENT_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_HOST_EXTERNAL_IP;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NAMESPACE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_NAME;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_OPEN_TELEMETRY_COLLECTOR;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_ORBIT_NAME;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_RECORD_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_SECRETS;
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
import io.fabric8.kubernetes.api.model.ResourceRequirements;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
import io.fabric8.kubernetes.client.utils.Serialization;
import io.javaoperatorsdk.operator.api.config.informer.Informer;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.quarkus.qute.Location;
import io.quarkus.qute.Template;
import io.quarkus.qute.TemplateInstance;
import io.quarkus.runtime.util.StringUtil;
import java.lang.invoke.MethodHandles;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.function.BiConsumer;
import java.util.function.Function;
import java.util.function.Predicate;
import org.jboss.logging.Logger;

@KubernetesDependent(
        informer =
                @Informer(
                        labelSelector =
                                "app.kubernetes.io/managed-by=solo-operator,app.kubernetes.io/part-of=consensus-node,app.kubernetes.io/component=deployment"))
public class CapsuleDeployment extends CRUDKubernetesDependentResource<Deployment, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-deploy";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    public static final String ERR_BOTH_HOSTPATH_PVC_NOT_ALLOWED =
            "a volume cannot have both HostPath and PersistentVolumeClaim specification: '%s'";
    public static final String ERR_DUPLICATE_VOLUME_SPECIFICATION =
            "volume specification is duplicated in persistentVolumeClaims and volumes sections for: %s";

    @Location("consensus-capsule/deployment.yaml")
    Template tpl;

    public CapsuleDeployment() {
        super(Deployment.class);
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
        Objects.requireNonNull(context);
        Objects.requireNonNull(context.getClient());

        log.infof(
                "Validating input for ConsensusCapsule: %s/%s ",
                primary.getMetadata().getNamespace(), primary.getMetadata().getName());
        // namespace must match with the orbit name
        final String orbit = primary.getSpec().getOrbit();
        if (!orbit.equals(primary.getMetadata().getNamespace())) {
            throw new IllegalArgumentException("namespace must match with the orbit name '%s', found '%s'"
                    .formatted(orbit, primary.getMetadata().getNamespace()));
        }
    }

    static Map<String, String> prepareLabels(ConsensusCapsule primary) {
        return Metadata.prepareConsensusNodeLabels(
                primary.getSpec().getNodeId(), primary.getSpec().getAccountId());
    }

    @Override
    protected Deployment desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        this.validateInput(primary, context);

        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = primary.getMetadata().getNamespace();
        final String orbit = primary.getSpec().getOrbit();

        final Deployment deployment = renderDeploymentTemplate(primary, context);
        final Map<String, String> labels = prepareLabels(primary);

        // return desired pod
        DeploymentBuilder deploymentBuilder = Metadata.applyStandardLabels(
                        deployment.edit().editMetadata(),
                        validName,
                        orbit,
                        Constants.COMPONENT_CONSENSUS_NODE,
                        Constants.COMPONENT_DEPLOYMENT,
                        labels)
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata();
        deploymentBuilder = Metadata.applyStandardLabels(
                        deploymentBuilder.editSpec().editTemplate().editMetadata(),
                        validName,
                        orbit,
                        Constants.COMPONENT_CONSENSUS_NODE,
                        Constants.COMPONENT_DEPLOYMENT,
                        labels)
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata()
                .endTemplate()
                .endSpec();

        this.setDesiredPodSpec(primary, deploymentBuilder);

        // set volumes
        final List<Volume> desiredVolumes = getDesiredPodVolumes(primary);
        for (final Volume v : desiredVolumes) {
            deploymentBuilder
                    .editSpec()
                    .editTemplate()
                    .editSpec()
                    .removeMatchingFromVolumes((s) -> Objects.equals(v.getName(), s.getName()))
                    .addToVolumes(v)
                    .endSpec()
                    .endTemplate()
                    .endSpec();
        }

        return deploymentBuilder.build();
    }

    private void setDesiredPodSpec(ConsensusCapsule primary, DeploymentBuilder deploymentBuilder) {
        copyPodProperties(primary.getSpec().getPodProperties(), deploymentBuilder);

        // force security context if it is not set already
        if (Objects.isNull(
                deploymentBuilder.build().getSpec().getTemplate().getSpec().getSecurityContext())) {
            deploymentBuilder
                    .editSpec()
                    .editTemplate()
                    .editSpec()
                    .withSecurityContext(SECURITY_CONTEXT_BUILDER.build())
                    .endSpec()
                    .endTemplate()
                    .endSpec();
        }
    }

    // FIXME: check property is set before using it
    DeploymentBuilder copyPodProperties(@NonNull final PodProperties props, final DeploymentBuilder builder) {
        Objects.requireNonNull(props);
        var spec = builder.editSpec().editTemplate().editSpec();
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
                Containers::getOpenTelemetryCollector,
                Containers::getRecordStreamSidecarFilesUploader,
                Containers::getRecordStreamUploader);
        for (Function<Containers, ?> getContainer : list) {
            Containers containers = props.getContainers();
            Object obj = null == containers ? null : getContainer.apply(containers);
            if (obj instanceof ConsensusNode container && match.test(container.getName())) {
                resources.accept(container.getName(), container.getResources());
            } else if (obj instanceof StreamUploader container && match.test(container.getName())) {
                resources.accept(container.getName(), container.getResources());
            } else if (obj instanceof OpenTelemetryCollector container && match.test(container.getName())) {
                resources.accept(container.getName(), container.getResources());
            }
        }
        return spec.endSpec().endTemplate().endSpec();
    }

    private TemplateInstance getTemplateWithDefaults(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        final Long nodeId = primary.getSpec().getNodeId();
        final String nodeName = Metadata.prepareNodeName(primary);
        final String accountId = primary.getSpec().getAccountId();
        final String namespace = primary.getMetadata().getNamespace();
        final String orbitName = primary.getSpec().getOrbit();

        final TemplateInstance templateInstance = tpl.instance();

        // set defaults
        templateInstance.data(TPL_VAR_ORBIT_NAME, orbitName);
        templateInstance.data(TPL_VAR_NAMESPACE, namespace);
        templateInstance.data(TPL_VAR_NODE_ID, nodeId);
        templateInstance.data(TPL_VAR_NODE_NAME, nodeName);
        templateInstance.data(TPL_VAR_ACCOUNT_ID, accountId);
        templateInstance.data(TPL_VAR_HOST_EXTERNAL_IP, primary.getSpec().getExternalAddress());
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

    protected Deployment renderDeploymentTemplate(ConsensusCapsule primary, Context<ConsensusCapsule> context) {

        // fetch public keys configmap
        final ConfigMap publicKeys = context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("public-keys")
                .get();

        final ConsensusNode cn =
                primary.getSpec().getPodProperties().getContainers().getConsensusNode();
        Objects.requireNonNull(cn, "spec.podProperties.containers.consensusNode is required");
        final Map<String, Optional<StreamUploader>> uploaderMap = this.prepareStreamUploaderContainers(primary);
        final Optional<OpenTelemetryCollector> oc = this.prepareOpenTelemetryCollector(primary);

        // set template variables
        TemplateInstance tplInstance = getTemplateWithDefaults(primary, context);
        tplInstance.data(TPL_VAR_CONSENSUS_NODE, cn);
        tplInstance.data(TPL_VAR_OPEN_TELEMETRY_COLLECTOR, oc.orElse(null));
        tplInstance.data(TPL_VAR_SECRETS, secretsWithDefaultKeyNames(primary));
        tplInstance.data(TPL_VAR_CONFIG, primary.getSpec().getConfigFolder());
        uploaderMap.forEach((k, v) -> tplInstance.data(k, v.orElse(null)));

        // return an instance of Pod
        return Serialization.unmarshal(tplInstance.render(), Deployment.class);
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

    protected Optional<OpenTelemetryCollector> prepareOpenTelemetryCollector(ConsensusCapsule primary) {
        final OpenTelemetryCollector oc =
                primary.getSpec().getPodProperties().getContainers().getOpenTelemetryCollector();

        return Optional.ofNullable(oc);
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
}
