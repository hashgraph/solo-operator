// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.networkoperation;

import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.util.List;
import java.util.function.BiConsumer;
import java.util.function.Function;

public class TestVerifyLayerHashes extends VerifyLayerHashes {
    public TestVerifyLayerHashes(
            final KubernetesClient kubernetesClient,
            final String namespace,
            final SoftwareVersionSource softwareVersionSource) {
        super(kubernetesClient, namespace, softwareVersionSource);
    }

    public static NetworkOperationStatus updateStatus(
            final HasMetadata resource,
            final NetworkOperationStatus status,
            final KubernetesClient kubernetesClient,
            final SoftwareVersionSource softwareVersionSource,
            final Function<NetworkOperationStatus, Condition> conditionFunction,
            final Function<SoftwareSourceStatus, List<ImageRepositoryStatus>> getStatusList,
            final BiConsumer<SoftwareSourceStatus, List<ImageRepositoryStatus>> setStatusList) {
        VerifyLayerHashes verifyLayerHashes = new TestVerifyLayerHashes(
                kubernetesClient, resource.getMetadata().getNamespace(), softwareVersionSource);
        return verifyLayerHashes.updateStatus(resource, status, conditionFunction, getStatusList, setStatusList);
    }

    @Override
    protected ImageCoordinates coordinates(final String image, final String... json) {
        return ImageCoordinates.from("http", image, json);
    }
}
