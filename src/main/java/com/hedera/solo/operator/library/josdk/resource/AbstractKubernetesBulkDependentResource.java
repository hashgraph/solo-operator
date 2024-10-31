// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.josdk.resource;

import static com.hedera.solo.operator.library.josdk.Resources.toMap;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import java.util.Map;

/** A {@code BulkDependentResource} that isn't required to create or delete, for use with JOSDK v4. */
public abstract class AbstractKubernetesBulkDependentResource<R extends HasMetadata, P extends HasMetadata>
        extends KubernetesDependentResource<R, P> implements BulkDependentResource<R, P> {
    public AbstractKubernetesBulkDependentResource(final Class<R> resourceType) {
        super(resourceType);
    }

    protected boolean create() {
        return false;
    }

    protected boolean delete() {
        return false;
    }

    /**
     * Default implementation, returns all secondary resources in event source sharing namespace with primary resource.
     */
    @Override
    public Map<String, R> getSecondaryResources(final P primary, final Context<P> context) {
        return toMap(eventSource()
                .orElseThrow()
                .list(primary.getMetadata().getNamespace())
                .toList());
    }

    @Override
    public R create(final R desired, final P primary, final Context<P> context) {
        return create() ? super.create(desired, primary, context) : null;
    }

    @Override
    public void deleteTargetResource(final P primary, final R resource, final String key, final Context<P> context) {
        if (delete()) {
            super.deleteTargetResource(primary, resource, key, context);
        }
    }
}
