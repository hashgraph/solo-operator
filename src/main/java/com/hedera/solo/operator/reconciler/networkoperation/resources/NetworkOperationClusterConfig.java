// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.k8s.KubeletConfig.clusterDomain;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static java.util.Collections.emptyList;

import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.consensus.genesis.AddressBook;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationReconciler;
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
import java.util.Map;
import java.util.Optional;
import java.util.TreeMap;

@KubernetesDependent
public class NetworkOperationClusterConfig extends KubernetesDependentResource<ConfigMap, NetworkOperation>
        implements Creator<ConfigMap, NetworkOperation>,
                ResourceDiscriminator<ConfigMap, NetworkOperation>,
                Updater<ConfigMap, NetworkOperation> {
    private final NetworkOperationReconciler reconciler;

    public static final String TEMPLATE = "cluster-config.yaml";

    public NetworkOperationClusterConfig(final NetworkOperationReconciler reconciler) {
        super(ConfigMap.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector(Constants.LABEL_SELECTOR_CONSENSUS_CONFIG)
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    protected ConfigMap desired(final NetworkOperation primary, final Context<NetworkOperation> context) {
        final String orbitName = primary.getSpec().getOrbit();
        return edit(context.getSecondaryResource(ConfigMap.class, this)
                        .orElseGet(() -> loadYaml(ConfigMap.class, getClass(), TEMPLATE)))
                .with(builder -> config(builder, primary, context))
                .with(builder -> Metadata.applyStandardLabels(
                                builder.editMetadata(),
                                Constants.CLUSTER_CONFIG_NAME,
                                orbitName,
                                Constants.COMPONENT_CONSENSUS_NODE,
                                Constants.COMPONENT_CONFIG_MAP,
                                null)
                        .withName(Constants.CLUSTER_CONFIG_NAME)
                        .withNamespace(primary.getMetadata().getNamespace()))
                .build();
    }

    @Override
    public Optional<ConfigMap> distinguish(
            final Class<ConfigMap> resource, final NetworkOperation primary, final Context<NetworkOperation> context) {
        return eventSource()
                .orElseThrow()
                .list(primary.getMetadata().getNamespace())
                .findAny();
    }

    record Node(Long nodeId, String accountId, String name, String internal, String external) {}

    Node node(final String domain, final ConsensusCapsule capsule) {
        String capsuleName = capsule.getMetadata().getName();
        String namespace = capsule.getMetadata().getNamespace();
        String external = capsule.getSpec().getExternalAddress();
        return new Node(
                capsule.getSpec().getNodeId(),
                capsule.getSpec().getAccountId(),
                Metadata.prepareNodeName(capsule),
                "%s.consensus.%s.svc%s".formatted(capsuleName, namespace, domain != null ? "." + domain : ""),
                external != null ? external : capsuleName + "-svc." + namespace);
    }

    List<Node> localNodes(final NetworkOperation primary, final Context<NetworkOperation> context) {
        String clusterDomainSuffix = clusterDomain(context.getClient());
        return reconciler.consensusCapsules(primary, context).values().stream()
                .map(capsule -> node(clusterDomainSuffix, capsule))
                .toList();
    }

    ConfigMapBuilder config(
            final ConfigMapBuilder builder, final NetworkOperation primary, final Context<NetworkOperation> context) {
        AddressBook addressBook = context.getSecondaryResource(Orbit.class)
                .orElseThrow()
                .getSpec()
                .getConsensus()
                .getGenesis()
                .getAddressBook();
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(addressBook.getLedgerId()).append("\n");
        sb.append("app, HederaNode.jar\n");
        Map<Long, String> addreses = new TreeMap<>();
        for (Node node : localNodes(primary, context)) {
            addreses.put(
                    node.nodeId,
                    "address, %d, %s, %s, 1, %s, 50111, %s, 50111, %s\n"
                            .formatted(
                                    node.nodeId, node.name, node.name, node.internal, node.external, node.accountId));
        }
        for (AddressBookEntry entry :
                Optional.ofNullable(addressBook.getExternalAddresses()).orElse(emptyList())) {
            addreses.put(
                    entry.getNodeId(),
                    "address, %d, %s, %s, %d, %s, %d, %s, %d, %s\n"
                            .formatted(
                                    entry.getNodeId(),
                                    entry.getNickName(),
                                    entry.getSelfName(),
                                    entry.getWeight(),
                                    entry.getInternalHostname(),
                                    entry.getInternalPort(),
                                    entry.getExternalHostname(),
                                    entry.getExternalPort(),
                                    entry.getNodeAccount()));
        }
        addreses.values().forEach(sb::append);
        sb.append("nextNodeId, ").append(addressBook.getNextNodeId());
        return builder.addToData("config.txt", sb.toString());
    }
}
