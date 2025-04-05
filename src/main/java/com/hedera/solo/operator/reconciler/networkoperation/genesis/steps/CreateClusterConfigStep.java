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

/** CreateClusterConfigStep creates the cluster config ConfigMap */
public class CreateClusterConfigStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String TEMPLATE = "cluster-config.yaml";
    public static final String CONDITION_TYPE = "ClusterConfigCreated";
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully created cluster config: %s/%s";
    public static final String FAIL_STATUS_MESSAGE = "Failed to create cluster config: %s";

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    public static final String POD_IPS_NOT_FOUND = "Pod IP not found for consensus node";

    private final CustomResourceManager customResourceManager;

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

            final String timestamp = timestamp();
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
                    .withType(CONDITION_TYPE)
                    .withStatus(ConditionStatus.True)
                    .withLastTransitionTime(timestamp)
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
        return capsule.getSpec().getInternalAddress() != null
                ? capsule.getSpec().getInternalAddress()
                : "%s.consensus.%s.svc%s"
                        .formatted(capsule.getMetadata().getName(), namespace, domain != null ? "." + domain : "");
    }

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

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
