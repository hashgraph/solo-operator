// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.List;
import java.util.Optional;

@KubernetesDependent
public class NetworkInitializationClusterConfig extends KubernetesDependentResource<ConfigMap, NetworkInitialization>
        implements Creator<ConfigMap, NetworkInitialization>,
                ResourceDiscriminator<ConfigMap, NetworkInitialization>,
                Updater<ConfigMap, NetworkInitialization> {

    public static final String TEMPLATE = "cluster-config.yaml";
    public static final String NAME = "cluster-config";

    public NetworkInitializationClusterConfig() {
        super(ConfigMap.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("app=consensus,type=config")
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    protected ConfigMap desired(final NetworkInitialization primary, final Context<NetworkInitialization> context) {
        return edit(context.getSecondaryResource(ConfigMap.class, this)
                        .orElseGet(() -> loadYaml(ConfigMap.class, getClass(), TEMPLATE)))
                .with(metadata -> metadata.editMetadata()
                        .withName(NAME)
                        .withNamespace(primary.getMetadata().getNamespace()))
                .with(builder -> config(builder, primary))
                .build();
    }

    @Override
    public Optional<ConfigMap> distinguish(
            final Class<ConfigMap> resource,
            final NetworkInitialization primary,
            final Context<NetworkInitialization> context) {
        return eventSource()
                .orElseThrow()
                .list(primary.getMetadata().getNamespace())
                .findAny();
    }

    record Node(String name, int weight, String internal, String external) {}

    List<Node> localNodes(final NetworkInitialization primary) {
        return primary.getStatus().getConsensusCapsules().stream()
                .map(name -> new Node(name, 1, name + ".consensus", name + "-svc"))
                .toList();
    }

    ConfigMapBuilder config(final ConfigMapBuilder builder, final NetworkInitialization primary) {
        // should include external address entries from orbit
        String chainId = builder.getData()
                .get("bootstrap.properties")
                .lines()
                .filter(s -> s.startsWith("contracts.chainId="))
                .findFirst()
                .orElseThrow()
                .substring(18);
        int nodeId = 0;
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(chainId).append("\n");
        sb.append("app, HederaNode.jar\n");
        for (Node node : localNodes(primary)) {
            sb.append("address, %d, %s, %s, 1, %s, 50111, %s, 50111, 0.0.%d\n"
                    .formatted(nodeId, node.name, node.name, node.internal, node.external, nodeId + 3));
            nodeId++;
        }
        sb.append("nextNodeId, ").append(nodeId);
        return builder.addToData("config.txt", sb.toString());
    }
}
