package com.hedera.solo.test.kubernetesbulkdependentresource;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import java.util.stream.StreamSupport;

//@KubernetesDependent
public abstract class KubernetesBulkDependentResource<R extends HasMetadata, P extends HasMetadata> extends KubernetesDependentResource<R, P>
        implements BulkDependentResource<R, P>
        , SecondaryToPrimaryMapper<R>
//    , Updater<R, P>
    , PrimaryToSecondaryMapper<P>
{
    public KubernetesBulkDependentResource(Class<R> resourceType) {
        super(resourceType);
    }

    protected abstract boolean create();
    protected abstract boolean delete();

    @Override
    protected R handleCreate(R desired, P primary, Context<P> context) {
        return create() ? super.handleCreate(desired, primary, context) : null;
    }

//    @Override
//    public Map<String, ConfigMap> desiredResources(Example primary, Context<Example> context) {
////        return Map.of("a", edit(new ConfigMap())
////                .with(b -> b
////                        .editMetadata()
////                        .withNamespace(primary.getMetadata().getNamespace())
////                        .withName("deleteme"))
////                .build());
//        return getSecondaryResources(primary, context).entrySet().stream()
//                .map(e -> Map.entry(e.getKey(), edit(e.getValue())
//                        .with(b -> b
//                                .addToData("updated", "true"))
//                        .build()))
//                .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue));
//    }

    protected Map<String, R> toMap(Iterable<R> iterable) {
        return StreamSupport.stream(iterable.spliterator(), false).collect(Collectors.toMap(ResourceID::toString, Function.identity()));
    }

    @Override
    public Map<String, R> getSecondaryResources(P primary, Context<P> context) {
        return toMap(context.getSecondaryResources(resourceType()));
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<R> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public R create(R desired, P primary, Context<P> context) {
        return create() ? super.create(desired, primary, context) : null;
    }

    @Override
    public void delete(P primary, Context<P> context) {
        if (delete()) {
            super.delete(primary, context);
        }
    }

    @Override
    protected void handleDelete(P primary, R secondary, Context<P> context) {
        if (delete()) {
            super.handleDelete(primary, secondary, context);
        }
    }

    @Override
    public void deleteTargetResource(P primary, R resource, String key, Context<P> context) {
        if (delete()) {
            super.deleteTargetResource(primary, resource, key, context);
        }
    }

//    @Override
//    public Set<ResourceID> toPrimaryResourceIDs(ConfigMap resource) {
//        return Collections.emptySet();
//    }

    @Override
    protected void onCreated(P primary, R created, Context<P> context) {
        if (create()) {
            super.onCreated(primary, created, context);
        }
    }

//    @Override
//    public Set<ResourceID> toSecondaryResourceIDs(Example primary) {
//        return Set.of(
//                new ResourceID("name0", primary.getMetadata().getNamespace()),
//                new ResourceID("name1", primary.getMetadata().getNamespace()),
//                new ResourceID("name2", primary.getMetadata().getNamespace())
//        );
//    }
}
