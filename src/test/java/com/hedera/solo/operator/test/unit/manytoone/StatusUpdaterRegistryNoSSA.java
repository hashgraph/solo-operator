package com.hedera.solo.operator.test.unit.manytoone;

import com.hedera.solo.operator.library.dependentresources.ManyToOneNoSSAKubernetesDependentResource;
import com.hedera.solo.operator.model.v1alpha1.StatusUpdater;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class StatusUpdaterRegistryNoSSA extends ManyToOneNoSSAKubernetesDependentResource<ConfigMap, StatusUpdater>
{
    public StatusUpdaterRegistryNoSSA() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap create(StatusUpdater primary) {
        return new ConfigMapBuilder()
                .editMetadata()
                .withNamespace(primary.getMetadata().getNamespace())
                .withName("status-updater-no-ssa")
                .endMetadata()
                .build();
    }
}
