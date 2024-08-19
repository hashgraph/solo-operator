package com.hedera.solo.test.labelselector;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.Set;

@KubernetesDependent(labelSelector = "test-label-selector")
public class LabelSelectorResource extends KubernetesDependentResource<ConfigMap, Example> implements
        SecondaryToPrimaryMapper<ConfigMap>,
        Updater<ConfigMap, Example>
{
    public LabelSelectorResource() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(Example primary, Context<Example> context) {
        return edit(new ConfigMap())
                .with(builder -> builder
                        .editMetadata()
                        .withName(primary.getMetadata().getName())
                        .withLabels(primary.getMetadata().getLabels()))
                .with(builder -> builder
                        .withData(Map.of("updated", "true")))
                .build();
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(ConfigMap resource) {
        return Set.of(ResourceID.fromResource(resource));
    }
}
