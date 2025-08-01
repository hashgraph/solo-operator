// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;

/** The HAProxySecret class is a Kubernetes dependent resource that manages the HAProxy secret. */
@KubernetesDependent
public class HAProxySecret extends KubernetesDependentResource<Secret, HAProxyCapsule>
        implements SecondaryToPrimaryMapper<Secret> {
    private final HAProxyCapsuleReconciler reconciler;

    /**
     * Constructs a new HAProxySecret instance with the specified HAProxyCapsuleReconciler.
     *
     * @param reconciler The HAProxyCapsuleReconciler to be used for managing the HAProxy secret.
     */
    public HAProxySecret(final HAProxyCapsuleReconciler reconciler) {
        this.reconciler = reconciler;
    }

    @Override
    public Optional<Secret> getSecondaryResource(final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        return eventSource()
                .orElseThrow()
                .list(primary.getMetadata().getNamespace())
                .filter(s -> Objects.equals(
                        primary.getSpec().getSecretName(), s.getMetadata().getName()))
                .findFirst();
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final Secret resource) {
        return reconciler
                .eventSourceContext()
                .getPrimaryCache()
                .list(
                        resource.getMetadata().getNamespace(),
                        p -> Objects.equals(
                                resource.getMetadata().getName(), p.getSpec().getSecretName()))
                .map(ResourceID::fromResource)
                .collect(Collectors.toSet());
    }
}
