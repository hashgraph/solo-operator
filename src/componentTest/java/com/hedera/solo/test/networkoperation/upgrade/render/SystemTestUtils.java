// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;

/** Utility class for system test related operations. */
public class SystemTestUtils {
    /** Private constructor to prevent instantiation of this utility class. */
    private SystemTestUtils() {}

    /**
     * Creates a Kubernetes custom resource with the given specification and status.
     *
     * @param client The Kubernetes client to interact with the cluster.
     * @param resource The custom resource to be created, containing both spec and status.
     * @param <Spec> The type of the specification of the custom resource.
     * @param <Status> The type of the status of the custom resource.
     * @param <T> The type of the custom resource that extends {@link CustomResource}.
     * @return The created custom resource after setting its status
     */
    public static <Spec, Status, T extends CustomResource<Spec, Status>> T create(
            final KubernetesClient client, final T resource) {
        T tmp = client.resource(resource).create();
        tmp.setStatus(resource.getStatus());
        return client.resource(tmp).updateStatus();
    }

    /**
     * Creates a new namespace in the Kubernetes cluster with a generated name based on the provided class.
     *
     * @param client The Kubernetes client to interact with the cluster.
     * @param aClass The class whose simple name will be used to generate the namespace name.
     * @return The name of the created namespace
     */
    public static String createNamespace(final KubernetesClient client, final Class<?> aClass) {
        return client.resource(edit(new Namespace())
                        .with(metadata -> metadata.editMetadata()
                                .withGenerateName(aClass.getSimpleName().toLowerCase() + "-"))
                        .build())
                .create()
                .getMetadata()
                .getName();
    }
}
