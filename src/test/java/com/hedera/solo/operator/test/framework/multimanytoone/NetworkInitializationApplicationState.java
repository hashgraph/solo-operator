package com.hedera.solo.operator.test.framework.multimanytoone;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class NetworkInitializationApplicationState extends AbstractApplicationState<NetworkInitialization> {
}
