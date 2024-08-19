package com.hedera.solo.operator.library.josdk.resource;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;
import java.util.stream.StreamSupport;

public abstract class AbstractKubernetesBulkDependentResource<R extends HasMetadata, P extends HasMetadata> extends KubernetesDependentResource<R, P>
        implements BulkDependentResource<R, P>
{
    public AbstractKubernetesBulkDependentResource(Class<R> resourceType) {
        super(resourceType);
    }

    protected boolean create() {
        return false;
    }

    protected boolean delete() {
        return false;
    }

    protected Map<String, R> toMap(Iterable<R> iterable) {
        return StreamSupport.stream(iterable.spliterator(), false).collect(Collectors.toMap(ResourceID::toString, Function.identity()));
    }

    @Override
    public Map<String, R> getSecondaryResources(P primary, Context<P> context) {
        return toMap(eventSource().orElseThrow().list().toList());
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<R> informerConfigurationBuilder() {
        if (this instanceof PrimaryToSecondaryMapper<?> mapper) {
            return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(mapper);
        }
        return super.informerConfigurationBuilder();
    }

    @Override
    public R create(R desired, P primary, Context<P> context) {
        return create() ? super.create(desired, primary, context) : null;
    }

    @Override
    protected R handleCreate(R desired, P primary, Context<P> context) {
        return create() ? super.handleCreate(desired, primary, context) : null;
    }

    @Override
    protected void onCreated(P primary, R created, Context<P> context) {
        if (create()) {
            super.onCreated(primary, created, context);
        }
    }

    @Override
    public void delete(P primary, Context<P> context) {
        if (delete()) {
            super.delete(primary, context);
        }
    }

    @Override
    public void deleteTargetResource(P primary, R resource, String key, Context<P> context) {
        if (delete()) {
            super.deleteTargetResource(primary, resource, key, context);
        }
    }

    @Override
    protected void handleDelete(P primary, R secondary, Context<P> context) {
        if (delete()) {
            super.handleDelete(primary, secondary, context);
        }
    }
}
