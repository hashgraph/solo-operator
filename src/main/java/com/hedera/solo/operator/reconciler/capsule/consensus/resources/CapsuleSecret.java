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

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Optional;
import java.util.Set;

@KubernetesDependent(resourceDiscriminator = CapsuleSecret.Discriminator.class)
public class CapsuleSecret extends CRUDKubernetesDependentResource<Secret, ConsensusCapsule> {

    public static final String TEMPLATE = "secret.yaml";
    public static final String NAME_SUFFIX = "-keys";

    public CapsuleSecret() {
        super(Secret.class);
    }

    @Override
    protected Secret desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        Secret secret = context.getClient()
                .resources(Secret.class)
                .inNamespace(primary.getMetadata().getNamespace())
                .withName(validName)
                .get();

        if (secret == null) {
            throw new IllegalArgumentException("Secret " + validName + " not found");
        }

        return secret;
    }

    public static class Discriminator implements ResourceDiscriminator<Secret, ConsensusCapsule> {
        @Override
        public Optional<Secret> distinguish(
                final Class<Secret> resource, final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {

            final String validName = validName(primary.getMetadata().getName(), NAME_SUFFIX);
            final Set<Secret> resources = context.getSecondaryResources(Secret.class);
            final Secret secret = context.getClient()
                    .resources(Secret.class)
                    .inNamespace(primary.getMetadata().getNamespace())
                    .withName(validName)
                    .get();

            return Optional.ofNullable(secret);
        }
    }
}
