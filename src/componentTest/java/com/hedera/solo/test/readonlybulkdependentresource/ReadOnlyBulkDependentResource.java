package com.hedera.solo.test.readonlybulkdependentresource;

import static com.hedera.solo.operator.library.builder.BuilderHelper.edit;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.ReconcileResult;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Collectors;

@KubernetesDependent
public class ReadOnlyBulkDependentResource extends KubernetesDependentResource<ConfigMap, Example>
        implements BulkDependentResource<ConfigMap, Example>
        , SecondaryToPrimaryMapper<ConfigMap>
    , Updater<ConfigMap, Example>
{
    public ReadOnlyBulkDependentResource() {
        super(ConfigMap.class);
    }

//    @Override
//    public ReconcileResult<ConfigMap> reconcile(Example primary, Context<Example> context) {
//        return super.reconcile(primary, context);
//    }
//
//    @Override
//    protected ReconcileResult<ConfigMap> reconcile(Example primary, ConfigMap actualResource, Context<Example> context) {
//        return super.reconcile(primary, actualResource, context);
//    }
//
//    @Override
//    protected ConfigMap handleCreate(ConfigMap desired, Example primary, Context<Example> context) {
//        return super.handleCreate(desired, primary, context);
//    }

//    @Override
//    public ReconcileResult<ConfigMap> reconcile(Example primary, Context<Example> context) {
//        return ReconcileResult.aggregatedResult(getSecondaryResources(primary, context).values().stream().map(ReconcileResult::noOperation).toList());
//    }

    @Override
    public Map<String, ConfigMap> desiredResources(Example primary, Context<Example> context) {
//        return Map.of("a", edit(new ConfigMap())
//                .with(b -> b
//                        .editMetadata()
//                        .withNamespace(primary.getMetadata().getNamespace())
//                        .withName("deleteme"))
//                .build());
        return getSecondaryResources(primary, context).entrySet().stream()
                .map(e -> Map.entry(e.getKey(), edit(e.getValue())
                        .with(b -> b
                                .addToData("updated", "true"))
                        .build()))
                .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue));
    }

    @Override
    public Map<String, ConfigMap> getSecondaryResources(Example primary, Context<Example> context) {
        return context.getSecondaryResourcesAsStream(ConfigMap.class).collect(Collectors.toMap(obj -> obj.getMetadata().getUid(), Function.identity()));
//        return context.getClient().configMaps().inNamespace(primary.getMetadata().getNamespace()).list().getItems().stream().filter(cm -> cm.getMetadata().getName().startsWith("name"))
//                .collect(Collectors.toMap(x -> x.getMetadata().getName(), x -> x));
    }

//    @Override
//    public void deleteTargetResource(Example primary, ConfigMap resource, String key, Context<Example> context) {}
//
//    @Override
//    public void delete(Example primary, Context<Example> context) {}
//
//    @Override
//    public ConfigMap create(ConfigMap desired, Example primary, Context<Example> context) {
//        return null;
//    }
    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<ConfigMap> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder()
                .withPrimaryToSecondaryMapper(p -> Set.of(
                        new ResourceID("name0", p.getMetadata().getNamespace()),
                        new ResourceID("name1", p.getMetadata().getNamespace()),
                        new ResourceID("name2", p.getMetadata().getNamespace())));
    }

    @Override
    public ConfigMap create(ConfigMap desired, Example primary, Context<Example> context) {
        return null; //super.create(desired, primary, context);
    }

    @Override
    public void delete(Example primary, Context<Example> context) {}

//    @Override
//    protected void handleDelete(Example primary, ConfigMap secondary, Context<Example> context) {
//        super.handleDelete(primary, secondary, context);
//    }

    @Override
    public void deleteTargetResource(Example primary, ConfigMap resource, String key, Context<Example> context) {}

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(ConfigMap resource) {
        return Set.of();
    }

    @Override
    protected void onCreated(Example primary, ConfigMap created, Context<Example> context) {
        //super.onCreated(primary, created, context);
    }
}
