package com.hedera.solo.test.readonlybulkdependentresource;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.ReconcileResult;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

// fixme: actually I want update only (not create or delete)

@KubernetesDependent
public class ReadOnlyBulkDependentResource extends KubernetesDependentResource<ConfigMap, Example>
        implements BulkDependentResource<ConfigMap, Example>
        , SecondaryToPrimaryMapper<ConfigMap>
{
    public ReadOnlyBulkDependentResource() {
        super(ConfigMap.class);
    }

//    @Override
//    public ReconcileResult<ConfigMap> reconcile(Example primary, Context<Example> context) {
//        return ReconcileResult.aggregatedResult(getSecondaryResources(primary, context).values().stream().map(ReconcileResult::noOperation).toList());
//    }

    @Override
    public Map<String, ConfigMap> desiredResources(Example primary, Context<Example> context) {
        return Map.of();
    }

    @Override
    public Map<String, ConfigMap> getSecondaryResources(Example primary, Context<Example> context) {
        return context.getClient().configMaps().inNamespace(primary.getMetadata().getNamespace()).list().getItems().stream().filter(cm -> cm.getMetadata().getName().startsWith("name"))
                .collect(Collectors.toMap(x -> x.getMetadata().getName(), x -> x));
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
    public Set<ResourceID> toPrimaryResourceIDs(ConfigMap resource) {
        return Set.of();
    }
}
