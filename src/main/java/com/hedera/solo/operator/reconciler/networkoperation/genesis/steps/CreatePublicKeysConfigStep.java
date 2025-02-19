// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleUtils.secretsWithDefaultKeyNames;
import static com.hedera.solo.operator.reconciler.common.Metadata.prepareNodeName;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Secrets;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.consensus.genesis.AddressBook;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Base64;
import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * CreatePublicKeysConfigStep creates collects the public keys of different capsules in the network from multiple
 * sources and creates a single configmap with all the public keys.
 */
public class CreatePublicKeysConfigStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    private final CustomResourceManager customResourceManager;
    public static final String CONDITION_TYPE = "PublicKeyConfigCreated";

    public CreatePublicKeysConfigStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        final String namespace = resource.getMetadata().getNamespace();
        final Orbit orbit = customResourceManager.orbitByName(resource.getSpec().getOrbit());
        if (orbit == null) {
            throw new ProcessorException(
                    "Orbit not found: %s".formatted(resource.getSpec().getOrbit()));
        }

        final List<ConsensusCapsule> consensusCapsules = customResourceManager.consensusCapsules(namespace);
        if (consensusCapsules.isEmpty()) {
            throw new ProcessorException("No consensus capsules found in orbit: %s"
                    .formatted(orbit.getMetadata().getName()));
        }

        final ConfigMap existing = customResourceManager.configMapByName(namespace, Constants.PUBLIC_KEYS_NAME);
        final ConfigMap desired = this.prepareDesiredConfigMap(resource, context, orbit, consensusCapsules, existing);

        if (existing == null) {
            context.getClient().resource(desired).create();
        } else {
            context.getClient().resource(desired).patch();
        }

        // add conditions to the status saying that the configmap has been created
        final Condition condition = new ConditionBuilder()
                .withType(CONDITION_TYPE)
                .withStatus(ConditionStatus.True)
                .withLastTransitionTime(Instant.now().toString())
                .withMessage("%s/%s created for %d consensus nodes"
                        .formatted(
                                desired.getMetadata().getNamespace(),
                                desired.getMetadata().getName(),
                                consensusCapsules.size()))
                .build();
        return edit(status).with(s -> s.addToConditions(condition)).build();
    }

    ConfigMap prepareDesiredConfigMap(
            final NetworkOperation primary,
            final Context<NetworkOperation> context,
            final Orbit orbit,
            final List<ConsensusCapsule> consensusCapsules,
            ConfigMap existing) {
        final String namespace = primary.getMetadata().getNamespace();
        final String orbitName = primary.getSpec().getOrbit();
        final AddressBook addressBook =
                orbit.getSpec().getConsensus().getGenesis().getAddressBook();

        final ConfigMap config = existing != null ? existing : new ConfigMap();
        return edit(config)
                .with(builder -> publicKeys(builder, namespace, consensusCapsules, addressBook))
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
            final ConfigMapBuilder builder,
            final String namespace,
            List<ConsensusCapsule> consensusCapsules,
            AddressBook addressBook) {
        Objects.requireNonNull(builder, "builder is required");
        Objects.requireNonNull(namespace, "namespace is required");
        Objects.requireNonNull(consensusCapsules, "consensusCapsules is required");
        Objects.requireNonNull(addressBook, "addressBook is required");

        // add public keys for internal addresses
        Base64.Decoder decoder = Base64.getDecoder();
        for (ConsensusCapsule capsule : consensusCapsules) {
            SecretResources capsuleSecrets = Optional.ofNullable(secretsWithDefaultKeyNames(capsule))
                    .map(Secrets::getSecretResources)
                    .orElse(null);

            if (capsuleSecrets != null) {
                String key = "s-public-" + prepareNodeName(capsule.getSpec().getNodeId()) + ".pem";
                KeyReference ref = capsuleSecrets.getSigningPublicCertificate();
                Secret secret = customResourceManager.secretByName(namespace, ref.getSecretName());
                if (secret == null) {
                    throw new ProcessorException("Secret not found: %s".formatted(ref.getSecretName()));
                }

                String value = secret.getData().get(ref.getKeyName());
                builder.addToData(key, new String(decoder.decode(value), StandardCharsets.UTF_8));
            }
        }

        // add public keys for external addresses
        for (AddressBookEntry entry : addressBook.getExternalAddresses()) {
            builder.addToData("s-public-" + entry.getSelfName() + ".pem", entry.getPublicLedgerKey());
        }

        return builder;
    }

    public static boolean isDone(NetworkOperationStatus status) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(
                        c -> c.getType().equals(CONDITION_TYPE) && c.getStatus().equals(ConditionStatus.True));
    }
}
