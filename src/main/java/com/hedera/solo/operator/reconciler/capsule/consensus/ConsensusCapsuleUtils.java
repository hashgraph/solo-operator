// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus;

import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Secrets;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;

/** The ConsensusCapsuleUtils class provides utility methods for working with ConsensusCapsule objects. */
public class ConsensusCapsuleUtils {
    /** Private constructor to prevent instantiation of the utility class. */
    private ConsensusCapsuleUtils() {}

    /**
     * Sets default key names for the given secret resources in a consensus capsule. If the key name is not already set,
     * it assigns a default key name based on the type of the key.
     *
     * @param consensusCapsule The ConsensusCapsule object containing the secrets to be updated.
     * @return The Secrets object associated with the given ConsensusCapsule
     */
    public static Secrets secretsWithDefaultKeyNames(final ConsensusCapsule consensusCapsule) {
        Secrets secrets = consensusCapsule.getSpec().getSecrets();
        if (secrets != null) {
            SecretResources secretResources = secrets.getSecretResources();
            if (secretResources != null) {
                setDefaultKeyName(secretResources.getGrpcTlsPrivateKey(), "tls.key");
                setDefaultKeyName(secretResources.getGrpcTlsPublicCertificate(), "tls.crt");
                setDefaultKeyName(secretResources.getSigningPrivateKey(), "tls.key");
                setDefaultKeyName(secretResources.getSigningPublicCertificate(), "tls.crt");
            }
        }
        return secrets;
    }

    /**
     * Sets a default key name for the given KeyReference if it is not already set.
     *
     * @param keyReference The KeyReference object to which the default key name will be assigned.
     * @param defaultKeyName The default key name to assign if the current key name is null.
     */
    private static void setDefaultKeyName(final KeyReference keyReference, final String defaultKeyName) {
        if (keyReference.getKeyName() == null) {
            keyReference.setKeyName(defaultKeyName);
        }
    }
}
