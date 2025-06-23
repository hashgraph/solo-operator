// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.k8s.KubeletConfig.clusterDomain;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_DEPLOYMENT;
import static com.hedera.solo.operator.reconciler.common.Constants.CONFIG_KEY_CONFIG_TXT;
import static com.hedera.solo.operator.reconciler.common.Constants.CONFIG_KEY_POD_IPS;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_COMPONENT;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_PART_OF;
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
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodList;
import io.fabric8.kubernetes.api.model.PodStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.TreeMap;
import org.jboss.logging.Logger;

/** The CreateClusterConfigStep class is responsible for creating the cluster configuration ConfigMap. */
public class CreateClusterConfigStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String TEMPLATE = "cluster-config.yaml";
    public static final StepConditionType CONDITION_TYPE = new StepConditionType("ConfigCreated", "ClusterConfig");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully created cluster config: %s/%s";
    public static final String FAIL_STATUS_MESSAGE = "Failed to create cluster config: %s";

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    public static final String POD_IPS_NOT_FOUND = "Pod IP not found for consensus node";

    private final CustomResourceManager customResourceManager;

    /**
     * Constructor for CreateClusterConfigStep.
     *
     * @param customResourceManager The CustomResourceManager to manage Kubernetes resources.
     */
    public CreateClusterConfigStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        try {
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(status, "NetworkOperationStatus status is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

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
            log.infof(
                    "Successfully updated cluster config: %s/%s",
                    desired.getMetadata().getNamespace(), desired.getMetadata().getName());
            final Condition condition = new ConditionBuilder()
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withType(CONDITION_TYPE.type())
                    .withReason(CONDITION_TYPE.reason())
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(timestamp())
                    .withMessage(SUCCESS_STATUS_MESSAGE.formatted(
                            desired.getMetadata().getNamespace(),
                            desired.getMetadata().getName()))
                    .build();

            return NetworkOperationStepUtil.prepareStatus(status, List.of(condition));
        } catch (Throwable t) {
            if (t.getMessage().contains(POD_IPS_NOT_FOUND)) {
                log.warn(t.getMessage());
            } else {
                log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            }

            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    /**
     * Prepares the desired ConfigMap for the cluster configuration based on the provided parameters.
     *
     * @param resource The NetworkOperation resource being reconciled.
     * @param context The Context object providing access to the Kubernetes client and other runtime information.
     * @param orbit The Orbit associated with the network operation.
     * @param consensusCapsules List of ConsensusCapsule objects representing the consensus nodes.
     * @param config Existing ConfigMap (can be null if creating a new one).
     * @return The desired ConfigMap to be created or updated
     */
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
                .with(builder -> preparePodIPs(builder, resource, consensusCapsules.size()))
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

    /**
     * Prepares the ConfigMap data for the config.txt key based on the provided address book and node addresses.
     *
     * @param builder The ConfigMapBuilder instance to which the data is added.
     * @param addressBook The AddressBook containing information about the nodes.
     * @param nodeAddressMap Map of node IDs to their respective addresses.
     * @return The updated ConfigMapBuilder with the config.txt data added
     */
    ConfigMapBuilder prepareConfigTxt(
            final ConfigMapBuilder builder, final AddressBook addressBook, final Map<Long, String> nodeAddressMap) {
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(addressBook.getLedgerId()).append("\n");
        sb.append("app, HederaNode.jar\n");
        nodeAddressMap.values().forEach(sb::append);
        return builder.addToData(CONFIG_KEY_CONFIG_TXT, sb.toString());
    }

    /**
     * Prepares a map of node IDs to their respective addresses based on the provided domain and consensus capsules.
     *
     * @param domain The cluster domain string.
     * @param consensusCapsules List of ConsensusCapsule objects representing the consensus nodes.
     * @param addressBook The AddressBook containing information about the nodes (optional).
     * @return A map of node IDs to their respective addresses
     */
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

    /**
     * Computes the external address for a consensus node based on the provided capsule and name information.
     *
     * @param capsule The ConsensusCapsule object representing the node.
     * @param name The name of the node.
     * @param namespace The Kubernetes namespace of the node.
     * @return The computed external address for the consensus node
     */
    public static String computeNodeExternalAddress(ConsensusCapsule capsule, String name, String namespace) {
        return capsule.getSpec().getExternalAddress() != null
                ? capsule.getSpec().getExternalAddress()
                : name + "-svc." + namespace;
    }

    /**
     * Computes the internal address for a consensus node based on the provided domain and capsule information.
     *
     * @param domain The cluster domain string.
     * @param capsule The ConsensusCapsule object representing the node.
     * @param namespace The Kubernetes namespace of the node.
     * @return The computed internal address for the consensus node
     */
    public static String computeNodeInternalAddress(String domain, ConsensusCapsule capsule, String namespace) {
        return capsule.getSpec().getInternalAddress() != null
                ? capsule.getSpec().getInternalAddress()
                : "%s.consensus.%s.svc%s"
                        .formatted(capsule.getMetadata().getName(), namespace, domain != null ? "." + domain : "");
    }

    /**
     * Prepares a ConfigMapBuilder by adding pod IPs to its data section based on the given primary
     * {@code NetworkOperation} and expected count.
     *
     * @param builder The ConfigMapBuilder instance to which pod IPs will be added.
     * @param primary The primary network operation containing namespace and other metadata.
     * @param expected The expected number of pod IPs that should be present in the specified namespace with the given
     *     labels.
     * @return The updated ConfigMapBuilder with pod IPs data
     * @throws ProcessorException If the number of pod IPs found does not match the expected count.
     */
    private ConfigMapBuilder preparePodIPs(
            final ConfigMapBuilder builder, final NetworkOperation primary, final int expected) {
        Map<String, String> labels = Map.of(
                LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                LABEL_K8S_PART_OF, COMPONENT_CONSENSUS_NODE,
                LABEL_K8S_COMPONENT, COMPONENT_DEPLOYMENT);

        PodList podList = customResourceManager.pods(primary.getMetadata().getNamespace(), labels);
        List<String> podIPs = extractPodIps(podList);
        if (podIPs.size() != expected) {
            throw new ProcessorException("%s [found %s of %s for %s/%s]"
                    .formatted(
                            POD_IPS_NOT_FOUND,
                            podIPs.size(),
                            expected,
                            primary.getMetadata().getNamespace(),
                            primary.getMetadata().getName()));
        }
        String data = String.join("\n", podIPs);

        return builder.addToData(CONFIG_KEY_POD_IPS, data);
    }

    /**
     * Extracts the IP addresses of pods that match the specified labels and are in a running state.
     *
     * @param podList The list of Pod objects from which to extract IP addresses.
     * @return A list of strings representing the IP addresses of the pods
     */
    private static List<String> extractPodIps(PodList podList) {
        return podList.getItems().stream()
                .filter(pod -> pod.getMetadata().getDeletionTimestamp() == null)
                .map(Pod::getStatus)
                .filter(Objects::nonNull)
                .filter(status -> "Running".equals(status.getPhase()))
                .filter(status -> status.getConditions().stream()
                        .anyMatch(c -> "Ready".equals(c.getType()) && "False".equals(c.getStatus())))
                .map(PodStatus::getPodIP)
                .filter(Objects::nonNull)
                .toList();
    }

    /**
     * Checks if the given {@code NetworkOperationStatus} indicates that the network operation is complete or successful
     * based on a specific condition type.
     *
     * @param status The status of the network operation to check.
     * @return True if the network operation is considered done, false otherwise
     */
    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
