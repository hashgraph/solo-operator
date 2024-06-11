package com.hedera.solo.operator.reconciler.xp;

import com.hedera.solo.operator.library.dependentresources.ManyToOneCRUDKubernetesDependentResourceSSA;
import com.hedera.solo.operator.model.v1alpha1.StatusUpdater;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent(useSSA = BooleanWithUndefined.TRUE, resourceDiscriminator = StatusUpdaterRegistry.class)
public class StatusUpdaterRegistry extends ManyToOneCRUDKubernetesDependentResourceSSA<ConfigMap, StatusUpdater>
{
    public StatusUpdaterRegistry() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap create(StatusUpdater primary) {
        return new ConfigMapBuilder()
                .editMetadata()
                .withNamespace(primary.getMetadata().getNamespace())
                .withName("status-updater")
                .endMetadata()
                .build();
    }
}
