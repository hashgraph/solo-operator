package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import java.lang.invoke.MethodHandles;
import java.util.Comparator;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@KubernetesDependent
public class NetworkInitializationClusterConfig extends KubernetesDependentResource<ConfigMap, NetworkInitialization>
implements
        Creator<ConfigMap, NetworkInitialization>,
        Updater<ConfigMap, NetworkInitialization>,
        PrimaryToSecondaryMapper<NetworkInitialization>
{
    Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    public static final String TEMPLATE = "cluster-config.yaml";
    public static final String NAME = "cluster-config";

    public NetworkInitializationClusterConfig() {
        super(ConfigMap.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("app=consensus,type=config")
                .build());
    }

    @Override
    protected ConfigMap desired(NetworkInitialization primary, Context<NetworkInitialization> context) {
        log.info("cluster-config:desired consensus-capsules {}", context.getSecondaryResourcesAsStream(ConsensusCapsule.class)
                .map(CustomResource::getMetadata).map(ObjectMeta::getName).toList());
        log.info("cluster-config:desired orbit {}", context.getSecondaryResourcesAsStream(Orbit.class)
                .map(CustomResource::getMetadata).map(ObjectMeta::getName).toList());
//        if (!active(primary)) {
//            throw new UpdateInactiveException();
//        }
        return edit(context.getSecondaryResource(ConfigMap.class).orElseGet(() -> loadYaml(ConfigMap.class, getClass(), TEMPLATE)))
                .with(builder -> builder
                        .editMetadata()
                        .withName(NAME)
                        .withNamespace(primary.getMetadata().getNamespace()))
                .with(builder -> config(builder, primary))
                .build();
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<ConfigMap> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(NetworkInitialization primary) {
        return eventSource().orElseThrow().list(primary.getMetadata().getNamespace()).map(ResourceID::fromResource).collect(Collectors.toSet());
    }

    record Node(String name, int weight, String internal, String external) {}
    List<Node> localNodes(NetworkInitialization primary) {
        return primary.getStatus().getConsensusCapsules().stream()
//        return context.getSecondaryResources(ConsensusCapsule.class).stream()
//                .map(CustomResource::getMetadata)
//                .map(ObjectMeta::getName)
//                .sorted()
                .map(name -> new Node(name, 1, name + ".consensus", name + "-svc"))
                .toList();
    }

    ConfigMapBuilder config(ConfigMapBuilder builder, NetworkInitialization primary) {
        // should include external address entries from orbit
        // should probably set the chainId in bootstrap.properties
        String chainId = builder
                .getData()
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
