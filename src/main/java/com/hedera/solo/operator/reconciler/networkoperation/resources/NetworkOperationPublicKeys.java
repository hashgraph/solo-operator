// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleUtils.secretsWithDefaultKeyNames;
import static com.hedera.solo.operator.reconciler.common.Metadata.prepareNodeName;

import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Secrets;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.consensus.genesis.AddressBook;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationReconciler;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.Optional;

@KubernetesDependent
public class NetworkOperationPublicKeys extends KubernetesDependentResource<ConfigMap, NetworkOperation>
        implements Creator<ConfigMap, NetworkOperation>,
                ResourceDiscriminator<ConfigMap, NetworkOperation>,
                Updater<ConfigMap, NetworkOperation> {
    private final NetworkOperationReconciler reconciler;

    public NetworkOperationPublicKeys(final NetworkOperationReconciler reconciler) {
        super(ConfigMap.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector(Constants.LABEL_SELECTOR_PUBLIC_KEYS)
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    protected ConfigMap desired(final NetworkOperation primary, final Context<NetworkOperation> context) {
        final String orbitName = primary.getSpec().getOrbit();
        return edit(context.getSecondaryResource(ConfigMap.class, this).orElseGet(ConfigMap::new))
                .with(builder -> publicKeys(builder, primary, context))
                .with(builder -> Metadata.applyStandardLabels(
                                builder.editMetadata(),
                                Constants.PUBLIC_KEYS_NAME,
                                orbitName,
                                Constants.COMPONENT_CONSENSUS_NODE,
                                Constants.COMPONENT_CONFIG_MAP,
                                null)
                        .withName(Constants.PUBLIC_KEYS_NAME)
                        .withNamespace(primary.getMetadata().getNamespace()))
                .build();
    }

    ConfigMapBuilder publicKeys(
            final ConfigMapBuilder builder, final NetworkOperation primary, final Context<NetworkOperation> context) {
        Base64.Decoder decoder = Base64.getDecoder();
        for (ConsensusCapsule capsule :
                reconciler.consensusCapsules(primary, context).values()) {
            SecretResources secrets = Optional.ofNullable(secretsWithDefaultKeyNames(capsule))
                    .map(Secrets::getSecretResources)
                    .orElse(null);
            if (secrets != null) {
                String key = "s-public-" + prepareNodeName(capsule.getSpec().getNodeId()) + ".pem";
                KeyReference ref = secrets.getSigningPublicCertificate();
                String value = context.getClient()
                        .secrets()
                        .inNamespace(primary.getMetadata().getNamespace())
                        .withName(ref.getSecretName())
                        .get()
                        .getData()
                        .get(ref.getKeyName());
                builder.addToData(key, new String(decoder.decode(value), StandardCharsets.UTF_8));
            }
        }
        AddressBook addressBook = context.getSecondaryResource(Orbit.class)
                .orElseThrow()
                .getSpec()
                .getConsensus()
                .getGenesis()
                .getAddressBook();
        for (AddressBookEntry entry : addressBook.getExternalAddresses()) {
            builder.addToData("s-public-" + entry.getSelfName() + ".pem", entry.getPublicLedgerKey());
        }
        return builder;
    }

    @Override
    public Optional<ConfigMap> distinguish(
            final Class<ConfigMap> resource, final NetworkOperation primary, final Context<NetworkOperation> context) {
        return eventSource()
                .orElseThrow()
                .list(primary.getMetadata().getNamespace())
                .findAny();
    }
}
