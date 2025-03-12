// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
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
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.jboss.logging.Logger;

/**
 * CreatePublicKeysConfigStep creates collects the public keys of different capsules in the network from multiple
 * sources and creates a single configmap with all the public keys.
 */
public class CreatePublicKeysConfigStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    private final CustomResourceManager customResourceManager;
    public static final String CONDITION_TYPE = "PublicKeyConfigCreated";
    static final String SUCCESS_STATUS_MESSAGE = "Successfully created public key secret: %s/%s";
    static final String FAIL_STATUS_MESSAGE = "Failed to create public key secret: %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    public CreatePublicKeysConfigStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");
        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            log.infof(
                    "Skipping creation of public key config: %s/%s",
                    resource.getMetadata().getNamespace(), Constants.PUBLIC_KEYS_NAME);
            return status;
        }

        final String timestamp = timestamp();
        try {
            final String namespace = resource.getMetadata().getNamespace();
            final Orbit orbit =
                    customResourceManager.orbitByName(resource.getSpec().getOrbit());
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
            final ConfigMap desired =
                    this.prepareDesiredConfigMap(resource, context, orbit, consensusCapsules, existing);

            log.infof(
                    "Creating public key config: %s/%s",
                    desired.getMetadata().getNamespace(), desired.getMetadata().getName());

            if (existing == null) {
                context.getClient().resource(desired).create();
            } else {
                context.getClient().resource(existing).patch(desired);
            }

            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withType(CONDITION_TYPE)
                            .withStatus(ConditionStatus.True)
                            .withLastTransitionTime(timestamp)
                            .withMessage(SUCCESS_STATUS_MESSAGE.formatted(
                                    resource.getMetadata().getNamespace(),
                                    resource.getMetadata().getName()))
                            .build()))
                    .build();
        } catch (final Exception ex) {
            log.error("Failed to create public key secret", ex);
            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withObservedGeneration(
                                            resource.getMetadata().getGeneration())
                                    .withType(CONDITION_TYPE)
                                    .withStatus(ConditionStatus.True)
                                    .withLastTransitionTime(timestamp)
                                    .withMessage(FAIL_STATUS_MESSAGE.formatted(
                                            resource.getMetadata().getNamespace(),
                                            resource.getMetadata().getName(),
                                            ex.getMessage()))
                                    .build())
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }
    }

    private ConfigMap prepareDesiredConfigMap(
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

    private ConfigMapBuilder publicKeys(
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
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
