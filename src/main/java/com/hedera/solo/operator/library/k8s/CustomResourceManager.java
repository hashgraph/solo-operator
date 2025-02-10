// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.k8s;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.Resource;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.util.List;
import java.util.stream.Stream;

/**
 * It exposes functionalities to retrieve or update resources using the Kubernetes API directly
 *
 * <p>FIXME we could probably refactor this class similar to how the K8 class in solo has been refactored recently
 */
@ApplicationScoped
public final class CustomResourceManager {
    private final KubernetesClient client;

    @Inject
    public CustomResourceManager(KubernetesClient client) {
        this.client = client;
    }

    /** Prepare and return the list of active resources that needs to be considered for sequencing. */
    public List<ConsensusCapsule> consensusCapsules(String namespace) {
        Stream<Resource<ConsensusCapsule>> resources =
                client.resources(ConsensusCapsule.class).inNamespace(namespace).resources();
        return resources.map(Resource::get).toList();
    }

    /**
     * Retrieve a ConfigMap by name.
     *
     * @return ConfigMap if exists, otherwise null
     */
    public ConfigMap configMapByName(String namespace, String name) {
        List<Resource<ConfigMap>> resources = client.resources(ConfigMap.class)
                .inNamespace(namespace)
                .resources()
                .toList();
        for (Resource<ConfigMap> r : resources) {
            if (r.get() != null && r.get().getMetadata().getName().equals(name)) {
                return r.get();
            }
        }

        return null;
    }

    /**
     * Retrieve an Orbit by name.
     *
     * @return Orbit if exists, otherwise null
     */
    public Orbit orbitByName(String name) {
        List<Resource<Orbit>> resources =
                client.resources(Orbit.class).inNamespace(name).resources().toList();
        for (Resource<Orbit> r : resources) {
            if (r.get() != null && r.get().getMetadata().getName().equals(name)) {
                return r.get();
            }
        }

        return null;
    }

    /**
     * Retrieve a secret by name.
     *
     * @return Secret if exists, otherwise null
     */
    public Secret secretByName(String namespace, String name) {
        List<Resource<Secret>> resources = client.resources(Secret.class)
                .inNamespace(namespace)
                .resources()
                .toList();
        for (Resource<Secret> r : resources) {
            if (r.get() != null && r.get().getMetadata().getName().equals(name)) {
                return r.get();
            }
        }

        return null;
    }
}
