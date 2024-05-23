package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.dependent.GarbageCollected;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;

public abstract class CRDKubernetesDependentResource<R extends HasMetadata, P extends HasMetadata>
        extends
        KubernetesDependentResource<R, P>
        implements Creator<R, P>, GarbageCollected<P> {

    public CRDKubernetesDependentResource(Class<R> resourceType) {
        super(resourceType);
    }
}
