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

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleSecret;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.Map;
import java.util.Objects;

@KubernetesDependent
public class HAProxySecret extends CRUDKubernetesDependentResource<Secret, HAProxyCapsule> {
    public static final String TEMPLATE = "secret.yaml";
    public static final String NAME_SUFFIX = "-haproxy-secrets";

    private String cachedHederaPem;

    public HAProxySecret() {
        super(Secret.class);
    }

    public static String prepareName(String nodeName) {
        return validName(nodeName, NAME_SUFFIX);
    }

    @Override
    protected Secret desired(final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String nodeName = primary.getSpec().getConsensus().getNode();
        final String haProxySecretName = prepareName(nodeName);
        final String hederaPem = prepareHederaPem(primary, context);

        return loadYaml(Secret.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .withName(haProxySecretName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .withData(Map.of("hedera.pem", hederaPem))
                .build();
    }

    /**
     * Prepare hedera.pem from corresponding consensus node secret
     *
     * @param primary HAProxyCapsule instance
     * @param context HAProxyCapsule context
     * @return String Base64 encoded hedera.pem
     * @throws IllegalArgumentException if secret is not found
     */
    private String prepareHederaPem(final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        if (Objects.nonNull(cachedHederaPem) && !cachedHederaPem.isEmpty()) {
            return cachedHederaPem;
        }

        final String nodeName = primary.getSpec().getConsensus().getNode();
        final String capsuleSecretName = CapsuleSecret.prepareName(nodeName);
        final Base64.Decoder decoder = Base64.getDecoder();
        final Base64.Encoder encoder = Base64.getEncoder();

        // fetch capsule secret of the consensus node
        Secret secret = context.getClient()
                .resources(Secret.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName(capsuleSecretName)
                .get();

        if (Objects.isNull(secret) || Objects.isNull(secret.getData())) {
            throw new IllegalArgumentException("Secret %s not found".formatted(capsuleSecretName));
        }

        // extract hedera.key
        final String hederaKeyEncoded = secret.getData().get("hedera-%s.key".formatted(nodeName));
        if (Objects.isNull(hederaKeyEncoded) || hederaKeyEncoded.isEmpty()) {
            throw new IllegalArgumentException("Secret %s does not have a hedera.key".formatted(capsuleSecretName));
        }
        final String hederaKey = new String(decoder.decode(hederaKeyEncoded), StandardCharsets.UTF_8);

        // extract hedera.crt
        final String hederaCrtEncoded = secret.getData().get("hedera-%s.crt".formatted(nodeName));
        if (Objects.isNull(hederaCrtEncoded) || hederaCrtEncoded.isEmpty()) {
            throw new IllegalArgumentException("Secret %s does not have a hedera.crt".formatted(capsuleSecretName));
        }
        final String hederaCrt = new String(decoder.decode(hederaCrtEncoded), StandardCharsets.UTF_8);

        // prepare hedera.pem
        final String hederaPemDecoded = "%s%s".formatted(hederaKey, hederaCrt);

        // cache
        // FIXME is there a better way other than caching
        cachedHederaPem = encoder.encodeToString(hederaPemDecoded.getBytes(StandardCharsets.UTF_8));

        return cachedHederaPem;
    }
}
