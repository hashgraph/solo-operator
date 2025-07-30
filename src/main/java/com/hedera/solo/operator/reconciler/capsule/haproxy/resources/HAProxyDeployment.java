// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.library.data.Digest;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;
import java.util.HashMap;
import java.util.Map;
import java.util.stream.Stream;

/**
 * The HAProxyDeployment class is responsible for reconciling the Kubernetes Deployment resource for a HAProxy capsule.
 *
 * <p>It configures and manages the HAProxy deployment based on the specifications provided in the HAProxyCapsule custom
 * resource.
 */
@KubernetesDependent
public class HAProxyDeployment extends CRUDKubernetesDependentResource<Deployment, HAProxyCapsule> {
    public static final String TEMPLATE = "deployment.yaml";
    public static final String NAME_SUFFIX = "-ha-deploy";
    private final HAProxyCapsuleReconciler reconciler;

    /**
     * Constructs a new HAProxyDeployment instance with the specified HAProxyCapsuleReconciler.
     *
     * @param reconciler The HAProxyCapsuleReconciler to be used for managing the HAProxy deployment.
     */
    public HAProxyDeployment(final HAProxyCapsuleReconciler reconciler) {
        super(Deployment.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<Deployment>()
                .withKubernetesDependentInformerConfig(InformerConfiguration.builder(Deployment.class)
                        .withLabelSelector((Constants.LABEL_SELECTOR_HAPROXY_APP))
                        .build())
                .build());
    }

    /**
     * Prepares the labels for the HAProxy deployment based on the primary HAProxyCapsule specification.
     *
     * @param primary The primary HAProxyCapsule instance used to extract necessary information.
     * @return A Map of string labels keyed by label names
     */
    static Map<String, String> prepareHAProxyLabels(HAProxyCapsule primary) {
        final Map<String, String> labels = new HashMap<>();
        labels.put(Constants.LABEL_APP, Constants.COMPONENT_HAPROXY);
        return labels;
    }

    @Override
    protected Deployment desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = metadata.getNamespace();

        final Map<String, String> labels = prepareHAProxyLabels(primary);

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
                        "none",
                        Constants.COMPONENT_HAPROXY,
                        Constants.COMPONENT_DEPLOYMENT,
                        labels)
                .addToLabels(labels)
                .addToAnnotations(
                        "checksum/secret",
                        Digest.SHA_384.hexOf(reconciler
                                .haProxySecret
                                .getSecondaryResource(primary, context)
                                .orElseThrow()
                                .getData()
                                .entrySet()
                                .stream()
                                .sorted(Map.Entry.comparingByKey())
                                .flatMap(entry -> Stream.of(
                                        entry.getKey().getBytes(),
                                        entry.getValue().getBytes()))))
                .endMetadata()
                .editSpec()
                .editMatchingContainer(c -> "haproxy".equalsIgnoreCase(c.getName()))
                .withEnv(primary.getSpec().getEnv())
                .withImage(primary.getSpec().getImage())
                .endContainer()
                .editMatchingVolume(vb -> "haproxy-cm".equals(vb.getName()))
                .editConfigMap()
                .withName(validName(metadata.getName(), HAProxyConfigMap.NAME_SUFFIX))
                .endConfigMap()
                .endVolume()
                .editMatchingVolume(vb -> "haproxy-secret".equals(vb.getName()))
                .editSecret()
                .withSecretName(primary.getSpec().getSecretName())
                .endSecret()
                .endVolume()
                .endSpec()
                .endTemplate()
                .endSpec()
                .build();
    }

    /**
     * The Ready condition implementation for the HAProxyDeployment. It checks if the HAProxy consensus service is
     * present in the context.
     */
    public static class Ready implements Condition<Deployment, HAProxyCapsule> {
        private final HAProxyCapsuleReconciler reconciler;

        /**
         * Constructs a new Ready instance with the specified HAProxyCapsuleReconciler.
         *
         * @param reconciler The HAProxyCapsuleReconciler to be used for checking conditions.
         */
        public Ready(final HAProxyCapsuleReconciler reconciler) {
            this.reconciler = reconciler;
        }

        @Override
        public boolean isMet(
                DependentResource<Deployment, HAProxyCapsule> dependentResource,
                HAProxyCapsule primary,
                Context<HAProxyCapsule> context) {
            return reconciler
                    .haProxySecret
                    .getSecondaryResource(primary, context)
                    .isPresent();
        }
    }
}
