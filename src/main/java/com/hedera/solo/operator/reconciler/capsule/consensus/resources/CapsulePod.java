// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

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
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_DEFAULT_MODE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_EVENT_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NAMESPACE;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_NODE_IDS;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_PUBLIC_KEY_FILES;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_RECORD_STREAM_UPLOADER;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.containers.ConsensusNode;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodBuilder;
import io.fabric8.kubernetes.api.model.PodSecurityContext;
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

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=consensus-node")
public class CapsulePod extends CRUDKubernetesDependentResource<Pod, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-pod";
    public static final String ACCOUNT_ID_LABEL = "account-id";

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
        Objects.requireNonNull(primary.getSpec().getPod());
        Objects.requireNonNull(primary.getSpec().getPod().getContainers());
        Objects.requireNonNull(primary.getSpec().getPod().getContainers().getConsensusNode());
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
                ACCOUNT_ID_LABEL,
                primary.getSpec().getAccountId());

        // return desired pod
        PodBuilder podBuilder = pod.edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(namespace)
                .addToLabels(labels)
                .endMetadata();

        podBuilder = this.setDesiredPodSpec(primary, podBuilder);

        return podBuilder.build();
    }

    private PodBuilder setDesiredPodSpec(ConsensusCapsule primary, PodBuilder podBuilder) {
        PodSecurityContext securityContext = primary.getSpec().getPod().getSecurityContext();
        if (securityContext == null) {
            securityContext = SECURITY_CONTEXT_BUILDER.build();
        }

        // TODO add other pod spec and associated test #263

        return podBuilder.editSpec().withSecurityContext(securityContext).endSpec();
    }

    private TemplateInstance getTemplateWithDefaults(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        final String nodeId = primary.getMetadata().getName();
        final String accountId = primary.getSpec().getAccountId();
        final String namespace = primary.getMetadata().getNamespace();
        final List<String> nodeIds = getNodeIds(context, namespace);

        final TemplateInstance templateInstance = tpl.instance();

        // set defaults
        templateInstance.data(TPL_VAR_NAMESPACE, namespace);
        templateInstance.data(TPL_VAR_NODE_ID, nodeId);
        templateInstance.data(TPL_VAR_NODE_IDS, nodeIds);
        templateInstance.data(TPL_VAR_ACCOUNT_ID, accountId);
        templateInstance.data(TPL_VAR_DEFAULT_MODE, DEFAULT_FILE_MODE);

        return templateInstance;
    }

    private Pod renderDeploymentTemplate(ConsensusCapsule primary, Context<ConsensusCapsule> context) {

        // fetch public keys configmap
        final ConfigMap publicKeys = context.getClient()
                .resources(ConfigMap.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName("public-keys")
                .get();

        final ConsensusNode cn = primary.getSpec().getPod().getContainers().getConsensusNode();
        final StreamUploader rsu = this.prepareRecordStreamUploader(primary);
        final StreamUploader esu = this.prepareEventStreamUploader(primary);
        final StreamUploader bu = this.prepareBackupUploader(primary);
        final StreamUploader rssu = this.prepareRecordStreamSidecarFilesUploader(primary);

        // set template variables
        TemplateInstance tplInstance = getTemplateWithDefaults(primary, context);
        tplInstance.data(TPL_VAR_PUBLIC_KEY_FILES, publicKeys.getData().keySet());
        tplInstance.data(TPL_VAR_CONSENSUS_NODE, cn);
        tplInstance.data(TPL_VAR_RECORD_STREAM_UPLOADER, rsu);
        tplInstance.data(TPL_VAR_EVENT_STREAM_UPLOADER, esu);
        tplInstance.data(TPL_VAR_BACKUP_UPLOADER, bu);
        tplInstance.data(TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER, rssu);

        // return an instance of Pod
        return Serialization.unmarshal(tplInstance.render(), Pod.class);
    }

    private StreamUploader prepareRecordStreamUploader(ConsensusCapsule primary) {
        final StreamUploader rsu = this.setStreamUploaderExtensions(
                primary.getSpec().getPod().getContainers().getRecordStreamUploader(),
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

    private StreamUploader prepareRecordStreamSidecarFilesUploader(ConsensusCapsule primary) {
        final StreamUploader rssu = this.setStreamUploaderExtensions(
                primary.getSpec().getPod().getContainers().getRecordStreamSidecarFilesUploader(),
                RECORD_STREAM_EXTENSION,
                RECORD_STREAM_SIG_EXTENSION);

        if (Objects.nonNull(rssu) && rssu.getConfig().getEnableCompression()) {
            rssu.getConfig().setStreamExtension("%s.gz".formatted(RECORD_STREAM_EXTENSION));
        }

        if (Objects.nonNull(rssu)) {
            rssu.getConfig()
                    .setBucketPath(RECORD_STREAM_BUCKET_PATH_TEMPLATE.formatted(
                            primary.getSpec().getAccountId()));
        }

        return rssu;
    }

    private StreamUploader prepareEventStreamUploader(ConsensusCapsule primary) {
        final StreamUploader esu = this.setStreamUploaderExtensions(
                primary.getSpec().getPod().getContainers().getEventsStreamUploader(),
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
        final StreamUploader bu = primary.getSpec().getPod().getContainers().getBackupUploader();

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

    private static List<String> getNodeIds(Context<ConsensusCapsule> context, String namespace) {
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
