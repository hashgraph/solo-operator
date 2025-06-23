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
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
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
    public static final StepConditionType CONDITION_TYPE = new StepConditionType("ConfigCreated", "PublicKeyConfig");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully created public key secret: %s/%s";
    public static final String FAIL_STATUS_MESSAGE = "Failed to create public key secret: %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    /**
     * Constructor for CreatePublicKeysConfigStep.
     *
     * @param customResourceManager CustomResourceManager instance for managing Kubernetes resources.
     */
    public CreatePublicKeysConfigStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        try {
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(status, "NetworkOperationStatus status is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

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

            if (existing == null) {
                log.infof(
                        "Creating public key config: %s/%s",
                        desired.getMetadata().getNamespace(),
                        desired.getMetadata().getName());
                context.getClient().resource(desired).create();
                log.infof(
                        "Successfully created public key config: %s/%s",
                        desired.getMetadata().getNamespace(),
                        desired.getMetadata().getName());
            } else {
                log.infof(
                        "Updating public key config: %s/%s",
                        desired.getMetadata().getNamespace(),
                        desired.getMetadata().getName());
                context.getClient().resource(existing).patch(desired);
                log.infof(
                        "Successfully updated public key config: %s/%s",
                        desired.getMetadata().getNamespace(),
                        desired.getMetadata().getName());
            }

            final Condition condition = new ConditionBuilder()
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withType(CONDITION_TYPE.type())
                    .withReason(CONDITION_TYPE.reason())
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(timestamp())
                    .withMessage(SUCCESS_STATUS_MESSAGE.formatted(
                            resource.getMetadata().getNamespace(),
                            resource.getMetadata().getName()))
                    .build();
            return NetworkOperationStepUtil.prepareStatus(status, List.of(condition));
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    /**
     * Prepares the desired configmap for public keys based on the provided resource and other inputs.
     *
     * @param primary NetworkOperation resource being processed.
     * @param context Context containing information about the Kubernetes client and other runtime data.
     * @param orbit Orbit specification to retrieve address book details.
     * @param consensusCapsules List of ConsensusCapsule instances to extract public keys from.
     * @param existing Existing configmap if it exists, or null if a new one is being created.
     * @return ConfigMapBuilder instance representing the desired state of the configmap
     */
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

    /**
     * Adds public keys to the configmap builder from consensus capsules and address book entries.
     *
     * @param builder ConfigMapBuilder instance to which public keys are added.
     * @param namespace Kubernetes namespace where the configmap is being created/updated.
     * @param consensusCapsules List of ConsensusCapsule instances to extract public keys from.
     * @param addressBook AddressBook specification containing external addresses with public keys.
     * @return ConfigMapBuilder instance with added public keys
     * @throws ProcessorException if a required secret is not found.
     */
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
            builder.addToData("s-public-" + entry.getSelfName() + ".pem", entry.getPublicSigningCertificate());
        }

        return builder;
    }

    /**
     * Checks if the step is done based on the current status of the network operation.
     *
     * @param status NetworkOperationStatus representing the current state of the network operation.
     * @return true if the step is successful, otherwise false
     */
    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
