package com.hedera.solo.operator.test.framework.applicationstate;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class ConsensusCapsuleApplicationState extends AbstractApplicationState<ConsensusCapsule> {
}
