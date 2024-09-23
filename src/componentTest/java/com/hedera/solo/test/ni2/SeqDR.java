package com.hedera.solo.test.ni2;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSequence;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;

public class SequenceDR extends CRUDKubernetesDependentResource<NetworkInitializationSequence, NetworkInitialization> {
    public SequenceDR() {
        super(NetworkInitializationSequence.class);
    }
}
