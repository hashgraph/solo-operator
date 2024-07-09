/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.library.dependentresources;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * Abstract dependent resource to manage many-to-one relation through garbage collected owner references, uses server
 * side apply Derived dependent resource class should implement create method and be configured to use itself as
 * resource discriminator with SSA.
 */
public abstract class ManyToOneKubernetesDependentResource<R extends HasMetadata, P extends HasMetadata>
        extends CRUDKubernetesDependentResource<R, P>
        implements PrimaryToSecondaryMapper<P>, ResourceDiscriminator<R, P> {
    public ManyToOneKubernetesDependentResource(Class<R> resourceType) {
        super(resourceType);
        setResourceDiscriminator(this);
    }

    protected abstract R createSecondaryResource(P primary);

    protected ResourceID secondaryResourceID(P primary) {
        return ResourceID.fromResource(createSecondaryResource(primary));
    }

    @Override
    protected R desired(P primary, Context<P> context) {
        R resource = distinguish(resourceType(), primary, context).orElseGet(() -> createSecondaryResource(primary));
        resource.getMetadata().setManagedFields(null);
        Cached.primaries(primary, context).forEach(resource::addOwnerReference);
        return resource;
    }

    public static AtomicInteger counter = new AtomicInteger();

    @Override
    public Optional<R> distinguish(Class<R> aClass, P primary, Context<P> context) {
        ResourceID resourceID = secondaryResourceID(primary);
        return context.getSecondaryResources(aClass).stream()
                .filter(r -> resourceID.equals(ResourceID.fromResource(r)))
                .findAny();
        //        Optional<R> fromCache = context.getSecondaryResources(aClass).stream().filter(r ->
        // id.equals(ResourceID.fromResource(r))).findAny();
        //        if (fromCache.isPresent()) {
        //            return fromCache;
        //        }
        //        counter.incrementAndGet();
        //        return
        // Optional.ofNullable(context.getClient().resources(aClass).inNamespace(primary.getMetadata().getNamespace()).withName(id.getName()).get());
    }

    @Override
    public Result<R> match(R actual, P primary, Context<P> context) {
        //        return Result.nonComputed(actual.hasOwnerReferenceFor(primary));
        return Result.nonComputed(Cached.primaries(primary, context).allMatch(actual::hasOwnerReferenceFor));
    }

    @Override
    protected boolean useSSA(Context<P> context) {
        return true;
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<R> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(P primary) {
        return Set.of(secondaryResourceID(primary));
    }
}
