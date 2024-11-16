// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.library.data.Digest;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;
import java.util.HashMap;
import java.util.Map;

@KubernetesDependent
public class HAProxyDeployment extends CRUDKubernetesDependentResource<Deployment, HAProxyCapsule> {
    public static final String TEMPLATE = "deployment.yaml";
    public static final String NAME_SUFFIX = "-ha-deploy";
    private final HAProxyCapsuleReconciler reconciler;

    public HAProxyDeployment(final HAProxyCapsuleReconciler reconciler) {
        super(Deployment.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<Deployment>()
                .withUseSSA(Boolean.FALSE)
                .withLabelSelector(Constants.LABEL_SELECTOR_HAPROXY)
                .build());
    }

    static Map<String, String> prepareHAProxyLabels(HAProxyCapsule primary) {
        final Map<String, String> labels = new HashMap<>();
        labels.put(Constants.LABEL_APP, Constants.COMPONENT_HAPROXY);
        labels.put(Constants.LABEL_NODE_ID, primary.getSpec().getNodeId().toString());
        labels.put(
                Constants.LABEL_NODE_NAME,
                Metadata.prepareNodeName(primary.getSpec().getNodeId()));
        return labels;
    }

    @Override
    protected Deployment desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = metadata.getNamespace();
        final String orbit = primary.getSpec().getOrbit();

        final Map<String, String> labels = prepareHAProxyLabels(primary);
        labels.put(
                Constants.COMPONENT_HAPROXY,
                Metadata.prepareNodeName(primary.getSpec().getNodeId()));

        final Deployment deployment = loadYaml(Deployment.class, getClass(), TEMPLATE);
        return Metadata.applyStandardLabels(
                        deployment
                                .edit()
                                .editMetadata()
                                .withName(validName)
                                .withNamespace(namespace)
                                .endMetadata()
                                .editSpec()
                                .editSelector()
                                .addToMatchLabels(labels)
                                .endSelector()
                                .editTemplate()
                                .editMetadata(),
                        validName,
                        orbit,
                        Constants.COMPONENT_HAPROXY,
                        Constants.COMPONENT_DEPLOYMENT,
                        labels)
                .addToLabels(labels)
                .addToAnnotations(
                        "checksum/secret",
                        Digest.of(reconciler
                                .haProxySecret
                                .getSecondaryResource(primary, context)
                                .orElseThrow()
                                .getData()))
                .endMetadata()
                .editSpec()
                .editMatchingContainer(c -> "haproxy".equalsIgnoreCase(c.getName()))
                .withEnv(
                        new EnvVar(
                                "HAPROXY_TARGET_IP",
                                reconciler
                                        .haProxyConsensusService
                                        .getSecondaryResource(primary, context)
                                        .map(Service::getMetadata)
                                        .map(ObjectMeta::getName)
                                        .orElseThrow(),
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
                .editMatchingVolume(vb -> "haproxy-secret".equals(vb.getName()))
                .editSecret()
                .withSecretName(validName(metadata.getName(), HAProxySecret.NAME_SUFFIX))
                .endSecret()
                .endVolume()
                .endSpec()
                .endTemplate()
                .endSpec()
                .build();
    }

    public static class Ready implements Condition<Deployment, HAProxyCapsule> {
        private final HAProxyCapsuleReconciler reconciler;

        public Ready(final HAProxyCapsuleReconciler reconciler) {
            this.reconciler = reconciler;
        }

        @Override
        public boolean isMet(
                DependentResource<Deployment, HAProxyCapsule> dependentResource,
                HAProxyCapsule primary,
                Context<HAProxyCapsule> context) {
            return reconciler
                    .haProxyConsensusService
                    .getSecondaryResource(primary, context)
                    .isPresent();
        }
    }
}
