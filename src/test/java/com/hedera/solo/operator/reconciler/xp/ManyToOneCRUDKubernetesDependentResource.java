package com.hedera.solo.operator.reconciler.xp;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import java.util.Optional;

public abstract class ManyToOneCRUDKubernetesDependentResource<R extends HasMetadata, P extends HasMetadata>
        extends CRUDKubernetesDependentResource<R, P>
        implements ResourceDiscriminator<R, P>
{
    public ManyToOneCRUDKubernetesDependentResource(Class<R> resourceType) {
        super(resourceType);
    }

    abstract R create(P primary);

    @Override
    protected R desired(P primary, Context<P> context) {
        R resource = distinguish(resourceType(), primary, context).orElseGet(() -> create(primary));
        resource.getMetadata().setManagedFields(null);
        context.getPrimaryCache().list().forEach(resource::addOwnerReference);
        return resource;
    }

    @Override
    public Optional<R> distinguish(Class<R> aClass, P primary, Context<P> context) {
        ResourceID id = ResourceID.fromResource(create(primary));
        return context.getSecondaryResources(aClass).stream().filter(r -> id.equals(ResourceID.fromResource(r))).findAny();
    }

    @Override
    public Result<R> match(R actual, P primary, Context<P> context) {
        return Result.nonComputed(context.getPrimaryCache().list().allMatch(actual::hasOwnerReferenceFor));
    }
}
