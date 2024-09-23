package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.lang.invoke.MethodHandles;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@KubernetesDependent
public class NetworkInitializationClusterConfig extends KubernetesDependentResource<ConfigMap, NetworkInitialization>
implements
        Creator<ConfigMap, NetworkInitialization>,
        Updater<ConfigMap, NetworkInitialization>
{
    Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    public static final String TEMPLATE = "cluster-config.yaml";
    public static final String NAME = "cluster-config";

    public NetworkInitializationClusterConfig() {
        super(ConfigMap.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .build());
    }

    @Override
    protected ConfigMap desired(NetworkInitialization primary, Context<NetworkInitialization> context) {
        log.info("cluster-config:desired consensus-capsules {}", context.getSecondaryResourcesAsStream(ConsensusCapsule.class)
                .map(CustomResource::getMetadata).map(ObjectMeta::getName).toList());
        log.info("cluster-config:desired orbit {}", context.getSecondaryResourcesAsStream(Orbit.class)
                .map(CustomResource::getMetadata).map(ObjectMeta::getName).toList());
        return loadYaml(ConfigMap.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .withName(NAME)
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .build();
    }

    record Node(String name, int weight, String internal, String external) {}

    ConfigMap config(ConfigMap configMap, Node... nodes) {
        // should probably set the chainId in bootstrap.properties
        String chainId = configMap
                .getData()
                .get("bootstrap.properties")
                .lines()
                .filter(s -> s.startsWith("contracts.chainId="))
                .findFirst()
                .get()
                .substring(18);
        int nodeId = 0;
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(chainId).append("\n");
        sb.append("app, HederaNode.jar\n");
        for (Node node : nodes) {
            sb.append("address, %d, %s, %s, 1, %s, 50111, %s, 50111, 0.0.%d\n"
                    .formatted(nodeId, node.name, node.name, node.internal, node.external, nodeId + 3));
            nodeId++;
        }
        sb.append("nextNodeId, ").append(nodeId);
        return configMap.edit().addToData("config.txt", sb.toString()).build();
    }
}
