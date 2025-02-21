// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.reconcile.ReconcileEvent.createOrUpdateEvent;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleUtils.secretsWithDefaultKeyNames;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Secrets;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;

public class CheckConsensusNodeSecretsStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String GRPC_TLS_PRIVATE_KEY = "GrpcTlsPrivateKey";
    public static final String GRPC_TLS_PUBLIC_CERTIFICATE = "GrpcTlsPublicCertificate";
    public static final String SIGNING_PRIVATE_KEY = "SigningPrivateKey";
    public static final String SIGNING_PUBLIC_CERTIFICATE = "SigningPublicCertificate";
    public static final String LEDGER_ENCRYPTION_KEY = "LedgerEncryptionKey";
    public static final String LEDGER_SIGNING_KEY = "LedgerSigningKey";

    public static final String CONDITION_TYPE = "CheckConsensusSecrets";
    public static final String SECRETS_VALID = "SecretsValid";

    private final CustomResourceManager customResourceManager;

    public CheckConsensusNodeSecretsStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed)) {
            return status;
        }

        final String namespace = resource.getMetadata().getNamespace();
        final List<ConsensusCapsule> consensusCapsules = customResourceManager.consensusCapsules(
                namespace, Map.of(Constants.LABEL_K8S_COMPONENT, Constants.COMPONENT_CONSENSUS_NODE));
        if (consensusCapsules.isEmpty()) {
            throw new ProcessorException("No consensus capsules found in namespace: %s".formatted(namespace));
        }

        boolean validationFailed = false;
        final List<Condition> conditions = new ArrayList<>();
        for (ConsensusCapsule capsule : consensusCapsules) {
            final List<Condition> validationFailures = validateSecretsOfConsensusCapsule(resource, status, capsule);
            if (!validationFailures.isEmpty()) {
                validationFailed = true;
            }
            conditions.addAll(validationFailures);
        }

        final NetworkOperationStatus.Phase phase =
                validationFailed ? NetworkOperationStatus.Phase.Failed : status.getPhase();

        if (validationFailed) {
            conditions.add(new ConditionBuilder()
                    .withType(CONDITION_TYPE)
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withStatus(ConditionStatus.False)
                    .withMessage("One or more secrets are not valid for consensus capsules")
                    .build());

            createOrUpdateEvent(
                    resource,
                    context,
                    new IllegalArgumentException("Secrets validation failed for consensus capsules"));
        } else {
            conditions.add(new ConditionBuilder()
                    .withType(CONDITION_TYPE)
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withStatus(ConditionStatus.True)
                    .withMessage("Secrets validation was successful for consensus capsules")
                    .build());
        }

        return edit(status)
                .with(s -> s.withConditions(conditions).withPhase(phase))
                .build();
    }

    private List<Condition> validateSecretsOfConsensusCapsule(
            NetworkOperation resource, NetworkOperationStatus status, ConsensusCapsule capsule) {
        final SecretResources secrets = Optional.ofNullable(secretsWithDefaultKeyNames(capsule))
                .map(Secrets::getSecretResources)
                .orElse(null);

        if (secrets == null) {
            return List.of();
        }

        final String namespace = capsule.getMetadata().getNamespace();
        final List<Condition> conditions = new ArrayList<>();

        if (secrets.getGrpcTlsPrivateKey() != null && !isValidSecret(namespace, secrets.getGrpcTlsPrivateKey())) {
            conditions.add(createValidationCondition(resource, status, capsule, GRPC_TLS_PRIVATE_KEY));
        }

        if (secrets.getGrpcTlsPublicCertificate() != null
                && !isValidSecret(namespace, secrets.getGrpcTlsPublicCertificate())) {
            conditions.add(createValidationCondition(resource, status, capsule, GRPC_TLS_PUBLIC_CERTIFICATE));
        }

        if (secrets.getSigningPrivateKey() != null && !isValidSecret(namespace, secrets.getSigningPrivateKey())) {
            conditions.add(createValidationCondition(resource, status, capsule, SIGNING_PRIVATE_KEY));
        }

        if (secrets.getSigningPublicCertificate() != null
                && !isValidSecret(namespace, secrets.getSigningPublicCertificate())) {
            conditions.add(createValidationCondition(resource, status, capsule, SIGNING_PUBLIC_CERTIFICATE));
        }

        if (secrets.getLedgerEncryptionKey() != null && !isValidSecret(namespace, secrets.getLedgerEncryptionKey())) {
            conditions.add(createValidationCondition(resource, status, capsule, LEDGER_ENCRYPTION_KEY));
        }

        if (secrets.getLedgerSigningKey() != null && !isValidSecret(namespace, secrets.getLedgerSigningKey())) {
            conditions.add(createValidationCondition(resource, status, capsule, LEDGER_SIGNING_KEY));
        }

        return conditions;
    }

    private boolean isValidSecret(String namespace, KeyReference keyRef) {
        Secret secret = customResourceManager.secretByName(namespace, keyRef.getSecretName());
        return secret != null && secret.getData().containsKey(keyRef.getKeyName());
    }

    /**
     * Create a condition for the validation of a specific key type
     *
     * <p>If a condition for the key type already exists, it will be updated with the new status. Otherwise, a new
     * condition will be created.
     *
     * @param resource - the network operation resource
     * @param status - the network operation status
     * @param capsule - the consensus capsule
     * @param keyType - the key type
     * @return the condition for the key type
     */
    private Condition createValidationCondition(
            NetworkOperation resource, NetworkOperationStatus status, ConsensusCapsule capsule, String keyType) {

        // check if a condition for the key type already exists, if so, update it
        if (status != null && status.getConditions() != null) {
            for (Condition c : status.getConditions()) {
                if (keyType.equals(c.getType())) {
                    return c.edit()
                            .withType(keyType)
                            .withReason(SECRETS_VALID)
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withStatus(ConditionStatus.False)
                            .withMessage("%s is not valid for capsule: %s "
                                    .formatted(keyType, capsule.getMetadata().getName()))
                            .build();
                }
            }
        }

        // create a new condition for the key type
        return new ConditionBuilder()
                .withType(keyType)
                .withObservedGeneration(resource.getMetadata().getGeneration())
                .withReason(SECRETS_VALID)
                .withStatus(ConditionStatus.False)
                .withMessage("%s is not valid for capsule: %s "
                        .formatted(keyType, capsule.getMetadata().getName()))
                .build();
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
