package com.hedera.solo.operator.library.dependentresources;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import java.util.Optional;

/**
 * Abstract dependent resource to manage many-to-one relation through garbage collected owner references, uses server side apply
 * Derived dependent resource class should implement create method and be configured to use itself as resource discriminator with SSA.
 * */
public abstract class ManyToOneCRUDKubernetesDependentResourceSSA<R extends HasMetadata, P extends HasMetadata>
        extends CRUDKubernetesDependentResource<R, P>
        implements ResourceDiscriminator<R, P>
{
    public ManyToOneCRUDKubernetesDependentResourceSSA(Class<R> resourceType) {
        super(resourceType);
    }

    protected abstract R create(P primary);

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
