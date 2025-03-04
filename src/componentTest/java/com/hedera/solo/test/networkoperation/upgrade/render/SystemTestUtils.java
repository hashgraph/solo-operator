// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;

public class SystemTestUtils {
    private SystemTestUtils() {}

    public static <Spec, Status, T extends CustomResource<Spec, Status>> T create(
            final KubernetesClient client, final T resource) {
        T tmp = client.resource(resource).create();
        tmp.setStatus(resource.getStatus());
        return client.resource(tmp).updateStatus();
    }

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
