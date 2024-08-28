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

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY;
import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler.HAS_LABEL_SELECTOR_HAPROXY;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
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
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final Base64.Decoder decoder = Base64.getDecoder();
        final Base64.Encoder encoder = Base64.getEncoder();
        BiFunction<Secret, String, String> get =
                (secret, name) -> new String(decoder.decode(secret.getData().get(name)), StandardCharsets.UTF_8);
        String pem = reconciler
                .haProxyConsensusSecret
                .getSecondaryResource(primary, context)
                .map(secret -> get.apply(secret, "hedera.key").trim() + "\n"
                        + get.apply(secret, "hedera.crt").trim() + "\n")
                .map(s -> encoder.encodeToString(s.getBytes(StandardCharsets.UTF_8)))
                .orElseThrow();

        return loadYaml(Secret.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
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
