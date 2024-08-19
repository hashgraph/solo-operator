package com.hedera.solo.test.kubernetesbulkdependentresource;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.processing.dependent.Updater;

public class KubernetesBulkDependentResourceUpdater extends KubernetesBulkDependentResource implements Updater<ConfigMap, Example> {
    public KubernetesBulkDependentResourceUpdater(final boolean create, final boolean delete) {
        super(create, delete);
    }
}
