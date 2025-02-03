// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus;

import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Secrets;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;

public class ConsensusCapsuleUtils {
    private ConsensusCapsuleUtils() {}

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

    private static void setDefaultKeyName(final KeyReference keyReference, final String defaultKeyName) {
        if (keyReference.getKeyName() == null) {
            keyReference.setKeyName(defaultKeyName);
        }
    }
}
