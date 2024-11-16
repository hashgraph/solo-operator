// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY;
import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler.HAS_LABEL_SELECTOR_HAPROXY;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.Map;
import java.util.Optional;
import java.util.function.BiFunction;

@KubernetesDependent
public class HAProxySecret extends CRUDKubernetesDependentResource<Secret, HAProxyCapsule>
        implements ResourceDiscriminator<Secret, HAProxyCapsule> {

    public static final String TEMPLATE = "secret.yaml";
    public static final String NAME_SUFFIX = "-ha-keys";
    private final HAProxyCapsuleReconciler reconciler;

    public HAProxySecret(final HAProxyCapsuleReconciler reconciler) {
        super(Secret.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<Secret>()
                .withLabelSelector(LABEL_SELECTOR_HAPROXY)
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    protected Secret desired(final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String nodeName = metadata.getName();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = metadata.getNamespace();
        final Base64.Decoder decoder = Base64.getDecoder();
        final Base64.Encoder encoder = Base64.getEncoder();
        BiFunction<Secret, String, String> get =
                (secret, name) -> new String(decoder.decode(secret.getData().get(name)), StandardCharsets.UTF_8);
        String pem = reconciler
                .haProxyConsensusSecret
                .getSecondaryResource(primary, context)
                .map(secret -> get.apply(secret, "hedera-%s.key".formatted(nodeName))
                                .trim() + "\n"
                        + get.apply(secret, "hedera-%s.crt".formatted(nodeName)).trim() + "\n")
                .map(s -> encoder.encodeToString(s.getBytes(StandardCharsets.UTF_8)))
                .orElseThrow();

        final Secret secret = loadYaml(Secret.class, getClass(), TEMPLATE);
        return Metadata.applyStandardLabels(
                        secret.edit().editMetadata(),
                        validName,
                        namespace,
                        Constants.COMPONENT_HAPROXY,
                        Constants.COMPONENT_SECRET,
                        Map.of(Constants.LABEL_NODE_NAME, nodeName))
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata()
                .addToData("hedera.pem", pem)
                .build();
    }

    @Override
    public Optional<Secret> distinguish(
            final Class<Secret> resource, final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        return context.getSecondaryResources(Secret.class).stream()
                .filter(HAS_LABEL_SELECTOR_HAPROXY)
                .findAny();
    }
}
