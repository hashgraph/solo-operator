// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.k8s.KubeletConfig.clusterDomain;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_DEPLOYMENT;
import static com.hedera.solo.operator.reconciler.common.Constants.CONFIG_KEY_CONFIG_TXT;
import static com.hedera.solo.operator.reconciler.common.Constants.CONFIG_KEY_POD_IPS;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_COMPONENT;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_PART_OF;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.SOLO_OPERATOR;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static java.util.Collections.emptyList;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.consensus.genesis.AddressBook;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodList;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.SortedSet;
import java.util.TreeMap;
import java.util.TreeSet;
import org.jboss.logging.Logger;

public class CreateClusterConfigStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String TEMPLATE = "cluster-config.yaml";
    public static final String CONDITION_TYPE = "ClusterConfigCreated";

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    public static final String POD_IPS_NOT_FOUND = "Pod IPs are not found for all consensus nodes";

    private final CustomResourceManager customResourceManager;

    public CreateClusterConfigStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");
        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            return status;
        }

        try {
            final String namespace = resource.getMetadata().getNamespace();
            final Orbit orbit =
                    customResourceManager.orbitByName(resource.getSpec().getOrbit());
            if (orbit == null) {
                throw new ProcessorException(
                        "Orbit not found: %s".formatted(resource.getSpec().getOrbit()));
            }

            final List<ConsensusCapsule> consensusCapsules = customResourceManager.consensusCapsules(namespace);
            if (consensusCapsules.isEmpty()) {
                throw new ProcessorException("No consensus capsules found in orbit: %s"
                        .formatted(orbit.getMetadata().getName()));
            }

            final ConfigMap existing = customResourceManager.configMapByName(namespace, Constants.CLUSTER_CONFIG_NAME);
            final ConfigMap desired =
                    this.prepareDesiredConfigMap(resource, context, orbit, consensusCapsules, existing);

            if (existing == null) {
                log.infof(
                        "Creating cluster config: %s/%s",
                        desired.getMetadata().getNamespace(),
                        desired.getMetadata().getName());
                context.getClient().resource(desired).create();
            } else {
                log.infof(
                        "Updating cluster config: %s/%s",
                        desired.getMetadata().getNamespace(),
                        desired.getMetadata().getName());
                context.getClient().resource(existing).patch(desired);
            }

            // add conditions to the status saying that the configmap has been created
            final Condition condition = new ConditionBuilder()
                    .withType(CONDITION_TYPE)
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(Instant.now().toString())
                    .withMessage("Successfully created config: %s/%s"
                            .formatted(
                                    desired.getMetadata().getNamespace(),
                                    desired.getMetadata().getName()))
                    .build();
            return edit(status).with(s -> s.addToConditions(condition)).build();
        } catch (Exception ex) {
            if (ex instanceof ProcessorException && ex.getMessage().contains(POD_IPS_NOT_FOUND)) {
                log.warnf(
                        "%s:%s: pod IPs are not available yet for consensus nodes %s; re-attempting during next reconcile loop"
                                .formatted(
                                        resource.getMetadata().getNamespace(),
                                        resource.getMetadata().getName(),
                                        resource.getStatus().getConsensusCapsules()),
                        ex);
                return status;
            }

            log.error("Failed to create cluster config", ex);
            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withType(CONDITION_TYPE)
                                    .withStatus(ConditionStatus.False)
                                    .withLastTransitionTime(Instant.now().toString())
                                    .withMessage("Failed to create cluster config: %s".formatted(ex.getMessage()))
                                    .build())
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }
    }

    ConfigMap prepareDesiredConfigMap(
            final NetworkOperation resource,
            final Context<NetworkOperation> context,
            final Orbit orbit,
            final List<ConsensusCapsule> consensusCapsules,
            ConfigMap config) {

        final String orbitName = resource.getSpec().getOrbit();
        if (config == null) {
            config = loadYaml(ConfigMap.class, getClass(), TEMPLATE);
        }

        String clusterDomain = clusterDomain(context.getClient());
        final AddressBook addressBook =
                orbit.getSpec().getConsensus().getGenesis().getAddressBook();
        Map<Long, String> nodeAddresses = prepareNodeAddressMap(clusterDomain, consensusCapsules, addressBook);

        return edit(config)
                .with(builder -> prepareConfigTxt(builder, addressBook, nodeAddresses))
                .with(builder -> preparePodIPs(builder, resource))
                .with(builder -> Metadata.applyStandardLabels(
                                builder.editMetadata(),
                                Constants.CLUSTER_CONFIG_NAME,
                                orbitName,
                                COMPONENT_CONSENSUS_NODE,
                                Constants.COMPONENT_CONFIG_MAP,
                                null)
                        .withName(Constants.CLUSTER_CONFIG_NAME)
                        .withNamespace(resource.getMetadata().getNamespace()))
                .build();
    }

    ConfigMapBuilder prepareConfigTxt(
            final ConfigMapBuilder builder, final AddressBook addressBook, final Map<Long, String> nodeAddressMap) {
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(addressBook.getLedgerId()).append("\n");
        sb.append("app, HederaNode.jar\n");
        nodeAddressMap.values().forEach(sb::append);
        return builder.addToData(CONFIG_KEY_CONFIG_TXT, sb.toString());
    }

    Map<Long, String> prepareNodeAddressMap(
            final String domain, final List<ConsensusCapsule> consensusCapsules, final AddressBook addressBook) {
        Map<Long, String> addresses = new TreeMap<>();

        // compute local consensus node addresses
        for (ConsensusCapsule capsule : consensusCapsules) {
            String nodeName = Metadata.prepareNodeName(capsule);
            String namespace = capsule.getMetadata().getNamespace();
            String internal = computeNodeInternalAddress(domain, capsule, namespace);
            String external = computeNodeExternalAddress(capsule, nodeName, namespace);
            Long nodeId = capsule.getSpec().getNodeId();
            String accountId = capsule.getSpec().getAccountId();
            Long weight = capsule.getSpec().getWeight();
            addresses.put(
                    nodeId,
                    "address, %d, %s, %s, %s, %s, 50111, %s, 50111, %s\n"
                            .formatted(nodeId, nodeName, nodeName, weight, internal, external, accountId));
        }

        // override with external addresses from the address book
        // Note address book may have the full list of nodes or just the external addresses
        // during network upgrade, we expect the address book to have the full list of nodes
        for (AddressBookEntry entry :
                Optional.ofNullable(addressBook.getExternalAddresses()).orElse(emptyList())) {
            addresses.put(
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

        return addresses;
    }

    public static String computeNodeExternalAddress(ConsensusCapsule capsule, String name, String namespace) {
        return capsule.getSpec().getExternalAddress() != null
                ? capsule.getSpec().getExternalAddress()
                : name + "-svc." + namespace;
    }

    public static String computeNodeInternalAddress(String domain, ConsensusCapsule capsule, String namespace) {
        return "%s.consensus.%s.svc%s"
                .formatted(capsule.getMetadata().getName(), namespace, domain != null ? "." + domain : "");
    }

    ConfigMapBuilder preparePodIPs(final ConfigMapBuilder builder, NetworkOperation primary) {
        Map<String, String> labels = Map.of(
                LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                LABEL_K8S_PART_OF, COMPONENT_CONSENSUS_NODE,
                LABEL_K8S_COMPONENT, COMPONENT_DEPLOYMENT);

        PodList podList = customResourceManager.pods(primary.getMetadata().getNamespace(), labels);
        List<String> podIps = extractPodIps(podList);
        if (podIps.isEmpty()
                || podIps.size() != primary.getStatus().getConsensusCapsules().size()) {
            throw new ProcessorException(("%s [ labels: %s, consensus capsules: %s")
                    .formatted(POD_IPS_NOT_FOUND, labels, primary.getStatus().getConsensusCapsules()));
        }

        String data = String.join("\n", podIps);

        return builder.addToData(CONFIG_KEY_POD_IPS, data);
    }

    private static List<String> extractPodIps(PodList podList) {
        // create a map of node id to the list of pods sorted by creation timestamp
        Map<String, SortedSet<Pod>> podMap = new HashMap<>();
        for (Pod pod : podList.getItems()) {
            if (pod.getMetadata().getCreationTimestamp() != null) {
                podMap.computeIfAbsent(
                                pod.getMetadata().getLabels().get(LABEL_NODE_ID),
                                k -> new TreeSet<Pod>(Comparator.comparing(
                                        p -> Instant.parse(p.getMetadata().getCreationTimestamp()))))
                        .add(pod);
            }
        }

        // extract the pod IPs from the last pod in the sorted set
        List<String> podIps = new ArrayList<>();
        for (SortedSet<Pod> pods : podMap.values()) {
            final Pod pod = pods.last();
            if (pod.getStatus() != null && pod.getStatus().getPodIP() != null) {
                podIps.add(pod.getStatus().getPodIP());
            } else {
                // all pods must have an IP, otherwise let's just wait for the next reconcile loop
                log.warnf(
                        "Pod IP not found for pod: %s/%s, waiting for next reconcile loop",
                        pod.getMetadata().getNamespace(), pod.getMetadata().getName());
                return emptyList();
            }
        }

        return podIps;
    }

    public static boolean isDone(NetworkOperationStatus status) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(
                        c -> c.getType().equals(CONDITION_TYPE) && c.getStatus().equals(ConditionStatus.True));
    }
}
