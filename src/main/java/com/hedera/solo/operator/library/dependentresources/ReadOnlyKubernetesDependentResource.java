package com.hedera.solo.operator.library.dependentresources;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.dependent.GarbageCollected;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;

public abstract class ReadOnlyKubernetesDependentResource<R extends HasMetadata, P extends HasMetadata>
        extends KubernetesDependentResource<R, P>
        implements GarbageCollected<P>
{
    public ReadOnlyKubernetesDependentResource(Class<R> resourceType) {
        super(resourceType);
    }
}
