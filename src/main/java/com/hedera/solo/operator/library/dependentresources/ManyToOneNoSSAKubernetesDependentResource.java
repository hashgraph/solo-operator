package com.hedera.solo.operator.library.dependentresources;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.ResourceUpdaterMatcher;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.updatermatcher.GenericResourceUpdaterMatcher;

/**
 * Abstract dependent resource to manage many-to-one relation through garbage collected owner references, uses server side apply
 * Derived dependent resource class should implement create method and be configured to use itself as resource discriminator without SSA.
 * */
public abstract class ManyToOneNoSSAKubernetesDependentResource<R extends HasMetadata, P extends HasMetadata>
        extends ManyToOneKubernetesDependentResource<R, P>
        implements ResourceUpdaterMatcher<R>
{
    ResourceUpdaterMatcher<R> updater = GenericResourceUpdaterMatcher.updaterMatcherFor(resourceType());

    public ManyToOneNoSSAKubernetesDependentResource(Class<R> resourceType) {
        super(resourceType);
    }

    @Override
    public boolean matches(R actual, R desired, Context<?> context) {
        return updater.matches(actual, desired, context) && Cached.primaries(actual, context).allMatch(actual::hasOwnerReferenceFor);
    }

    @Override
    public R updateResource(R actual, R desired, Context<?> context) {
        R updated = updater.updateResource(actual, desired, context);
        Cached.primaries(updated, context).forEach(updated::addOwnerReference);
        return updated;
    }

    @Override
    protected boolean useSSA(Context<P> context) {
        return false;
    }
}
