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

/**
 * The TestVerifyLayerHashes class extends the VerifyLayerHashes abstract class. It provides a concrete implementation
 * of verifying layer hashes for network operations, utilizing KubernetesClient for interacting with the Kubernetes API.
 */
public class TestVerifyLayerHashes extends VerifyLayerHashes {
    /**
     * Constructs a new TestVerifyLayerHashes instance.
     *
     * @param kubernetesClient The Kubernetes client used to interact with the Kubernetes cluster.
     * @param namespace The Kubernetes namespace where the network operation is deployed.
     * @param softwareVersionSource The source from which to retrieve software version information.
     */
    public TestVerifyLayerHashes(
            final KubernetesClient kubernetesClient,
            final String namespace,
            final SoftwareVersionSource softwareVersionSource) {
        super(kubernetesClient, namespace, softwareVersionSource);
    }

    /**
     * Updates the status of a network operation by verifying layer hashes.
     *
     * @param resource The Kubernetes resource (metadata and spec) representing the network operation.
     * @param status The current status of the network operation which will be updated.
     * @param kubernetesClient The Kubernetes client used to interact with the Kubernetes cluster.
     * @param softwareVersionSource The source from which to retrieve software version information.
     * @param conditionFunction A function that creates a Condition object based on the current status.
     * @param getStatusList A function that retrieves the list of image repository statuses for the software source
     *     status.
     * @param setStatusList A BiConsumer that sets the list of image repository statuses for the software source status.
     * @return The updated NetworkOperationStatus with the verified conditions
     */
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
