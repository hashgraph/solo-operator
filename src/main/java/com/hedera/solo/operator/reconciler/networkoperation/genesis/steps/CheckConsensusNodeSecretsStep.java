// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis.steps;

import static com.hedera.solo.operator.library.reconcile.ReconcileEvent.createOrUpdateEvent;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
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
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import org.jboss.logging.Logger;

/**
 * CheckConsensusNodeSecretsStep checks if the consensus node secrets are valid
 *
 * <p>It validates the following secrets for each consensus node:
 *
 * <ul>
 *   <li>GrpcTlsPrivateKey
 *   <li>GrpcTlsPublicCertificate
 *   <li>SigningPrivateKey
 *   <li>SigningPublicCertificate
 *   <li>LedgerEncryptionKey
 *   <li>LedgerSigningKey
 * </ul>
 *
 * <p>If the secrets are not valid, the step will fail and the network operation status will be updated as Failed.
 */
public class CheckConsensusNodeSecretsStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String GRPC_TLS_PRIVATE_KEY = "GrpcTlsPrivateKey";
    public static final String GRPC_TLS_PUBLIC_CERTIFICATE = "GrpcTlsPublicCertificate";
    public static final String SIGNING_PRIVATE_KEY = "SigningPrivateKey";
    public static final String SIGNING_PUBLIC_CERTIFICATE = "SigningPublicCertificate";
    public static final String LEDGER_ENCRYPTION_KEY = "LedgerEncryptionKey";
    public static final String LEDGER_SIGNING_KEY = "LedgerSigningKey";

    public static final String CONDITION_TYPE = "CheckConsensusNodeSecrets";
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully validated consensus node secrets";
    public static final String FAIL_STATUS_MESSAGE = "Failed to validate consensus node secrets'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    public CheckConsensusNodeSecretsStep(CustomResourceManager customResourceManager) {
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

            final String timestamp = timestamp();
            final String namespace = resource.getMetadata().getNamespace();
            final List<ConsensusCapsule> consensusCapsules = customResourceManager.consensusCapsules(
                    namespace, Map.of(Constants.LABEL_K8S_COMPONENT, Constants.COMPONENT_CONSENSUS_NODE));
            if (consensusCapsules.isEmpty()) {
                throw new ProcessorException("No consensus capsules found in namespace: %s".formatted(namespace));
            }

            for (ConsensusCapsule capsule : consensusCapsules) {
                final List<Condition> conditions = validateSecretsOfConsensusCapsule(resource, status, capsule);
                if (!conditions.isEmpty()) {
                    // fail fast as soon as secrets are invalid for a consensus node
                    createOrUpdateEvent(resource, context, new IllegalArgumentException(FAIL_STATUS_MESSAGE));
                    conditions.add(new ConditionBuilder()
                            .withType(CONDITION_TYPE)
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withStatus(ConditionStatus.False)
                            .withLastTransitionTime(timestamp)
                            .withMessage(FAIL_STATUS_MESSAGE)
                            .build());
                    return NetworkOperationStepUtil.prepareStatus(
                            status, conditions, NetworkOperationStatus.Phase.Failed);
                }
            }

            log.infof("%s: %s", SUCCESS_STATUS_MESSAGE, consensusCapsules);
            final Condition condition = new ConditionBuilder()
                    .withType(CONDITION_TYPE)
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(timestamp)
                    .withMessage(SUCCESS_STATUS_MESSAGE)
                    .build();
            return NetworkOperationStepUtil.prepareStatus(status, List.of(condition));
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
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
            conditions.add(secretInvalidCondition(resource, status, capsule, GRPC_TLS_PRIVATE_KEY));
        }

        if (secrets.getGrpcTlsPublicCertificate() != null
                && !isValidSecret(namespace, secrets.getGrpcTlsPublicCertificate())) {
            conditions.add(secretInvalidCondition(resource, status, capsule, GRPC_TLS_PUBLIC_CERTIFICATE));
        }

        if (secrets.getSigningPrivateKey() != null && !isValidSecret(namespace, secrets.getSigningPrivateKey())) {
            conditions.add(secretInvalidCondition(resource, status, capsule, SIGNING_PRIVATE_KEY));
        }

        if (secrets.getSigningPublicCertificate() != null
                && !isValidSecret(namespace, secrets.getSigningPublicCertificate())) {
            conditions.add(secretInvalidCondition(resource, status, capsule, SIGNING_PUBLIC_CERTIFICATE));
        }

        if (secrets.getLedgerEncryptionKey() != null && !isValidSecret(namespace, secrets.getLedgerEncryptionKey())) {
            conditions.add(secretInvalidCondition(resource, status, capsule, LEDGER_ENCRYPTION_KEY));
        }

        if (secrets.getLedgerSigningKey() != null && !isValidSecret(namespace, secrets.getLedgerSigningKey())) {
            conditions.add(secretInvalidCondition(resource, status, capsule, LEDGER_SIGNING_KEY));
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
     * @param resource - the network operation resource
     * @param status - the network operation status
     * @param capsule - the consensus capsule
     * @param keyType - the key type
     * @return the condition for the key type
     */
    private Condition secretInvalidCondition(
            NetworkOperation resource, NetworkOperationStatus status, ConsensusCapsule capsule, String keyType) {
        final String nodeName = Metadata.prepareNodeName(capsule.getSpec().getNodeId());
        final String conditionType = "%s-%s".formatted(nodeName, keyType);
        return new ConditionBuilder()
                .withType(conditionType)
                .withObservedGeneration(resource.getMetadata().getGeneration())
                .withLastTransitionTime(timestamp())
                .withStatus(ConditionStatus.False)
                .withMessage("%s is not valid for consensus node: %s ".formatted(keyType, nodeName))
                .build();
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
