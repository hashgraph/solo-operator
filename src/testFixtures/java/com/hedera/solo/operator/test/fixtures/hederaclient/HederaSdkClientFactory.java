// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.hederaclient;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_CONSENSUS_NODE_APP;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_HAPROXY_APP;

import com.google.protobuf.ByteString;
import com.hedera.hashgraph.sdk.Client;
import com.hedera.hashgraph.sdk.NodeAddressBook;
import com.hedera.hashgraph.sdk.proto.AccountID;
import com.hedera.hashgraph.sdk.proto.ServiceEndpoint;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.LocalPortForward;
import io.fabric8.kubernetes.client.dsl.PortForwardable;
import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.security.cert.CertificateFactory;
import java.util.Arrays;
import java.util.Base64;
import java.util.Collection;
import java.util.Collections;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeoutException;
import java.util.function.Predicate;
import java.util.stream.Collectors;
import java.util.stream.Stream;

/**
 * Factory class to create Hedera SDK client context for both direct and HAProxy services using Kubernetes cluster
 * resources.
 */
public class HederaSdkClientFactory {
    public static final String DEFAULT_ACCOUNT_ID = "0.0.2";
    public static final String DEFAULT_PRIVATE_KEY =
            "302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137";
    private final KubernetesClient kubernetesClient;
    private final String namespace;

    /**
     * Constructor to initialize the factory with a Kubernetes client and namespace.
     *
     * @param kubernetesClient The Kubernetes client instance for interacting with the cluster.
     * @param namespace The Kubernetes namespace where Hedera services are deployed.
     */
    public HederaSdkClientFactory(final KubernetesClient kubernetesClient, final String namespace) {
        this.kubernetesClient = kubernetesClient;
        this.namespace = namespace;
    }

    /** Interface for port forwarding management. */
    public interface PortForward {
        /**
         * Creates a list of local port forwards for the given PortForwardable resource.
         *
         * @param portForwardable The PortForwardable resource to create local port forwards for.
         * @return A list of LocalPortForward instances representing the local port forwards
         */
        List<LocalPortForward> create(final PortForwardable portForwardable);
    }

    /**
     * Creates a client context for direct service communication with the consensus node.
     *
     * @param consensusNode The name of the consensus node service.
     * @param secure True if encryption is required, otherwise false.
     * @return A ClientContext instance for direct Hedera SDK client setup
     */
    public ClientContext directContextCreator(final String consensusNode, boolean secure) {
        return createClientContext(certificates(), pubKeys(), direct(), consensusNode, secure);
    }

    /**
     * Static factory method to create a context creator for direct service communication.
     *
     * @param certificates A map of certificate data indexed by node names.
     * @param pubKeys A map of public keys indexed by node names.
     * @return A function that creates ClientContext instances based on the factory and additional parameters
     */
    public static HederaSdkClientContextFactory directContextCreator(
            final Map<String, byte[]> certificates, final Map<String, String> pubKeys) {
        return (factory, consensusNode, secure) ->
                factory.createClientContext(certificates, pubKeys, factory.direct(), consensusNode, secure);
    }

    /**
     * Creates a client context for HAProxy service communication with the consensus node.
     *
     * @param consensusNode The name of the consensus node service.
     * @param secure True if encryption is required, otherwise false.
     * @return A ClientContext instance for HAProxy Hedera SDK client setup
     */
    public ClientContext haproxyContextCreator(final String consensusNode, boolean secure) {
        return createClientContext(certificates(), pubKeys(), haproxy(), consensusNode, secure);
    }

    /**
     * Private method to create a Hedera SDK client context.
     *
     * @param certificates A map of certificate data indexed by node names.
     * @param pubKeys A map of public keys indexed by node names.
     * @param nodeService A map of service names indexed by node names.
     * @param consensusNode The name of the consensus node service.
     * @param secure True if encryption is required, otherwise false.
     * @return A ClientContext instance for Hedera SDK client setup
     */
    ClientContext createClientContext(
            final Map<String, byte[]> certificates,
            final Map<String, String> pubKeys,
            final Map<String, String> nodeService,
            final String consensusNode,
            final boolean secure) {
        return getClientContext(certificates, pubKeys, nodeService, consensusNode::equals, resource -> (secure
                        ? Stream.of(50212)
                        : Stream.of(50211))
                .map(resource::portForward)
                .toList());
    }

    /**
     * Private method to get a Hedera SDK client context.
     *
     * @param certificates A map of certificate data indexed by node names.
     * @param pubKeys A map of public keys indexed by node names.
     * @param nodeService A map of service names indexed by node names.
     * @param predicate to select the appropriate services for client context creation.
     * @param portForward A function to create local port forwards for selected services.
     * @return A ClientContext instance for Hedera SDK client setup
     * @throws RuntimeException if there is an error in generating the client.
     */
    ClientContext getClientContext(
            final Map<String, byte[]> certificates,
            final Map<String, String> pubKeys,
            final Map<String, String> nodeService,
            final Predicate<String> predicate,
            final PortForward portForward) {
        try {
            return getClient(certificates, pubKeys, getPortForwarding(nodeService, predicate, portForward));
        } catch (Exception exception) {
            throw new RuntimeException(exception);
        }
    }

    /**
     * Private method to get port forwarding configurations for selected services.
     *
     * @param nodeService A map of service names indexed by node names.
     * @param select A predicate to select the appropriate services for port forwarding.
     * @param portForward A function to create local port forwards.
     * @return A map of local port forwards indexed by service names
     */
    Map<String, List<LocalPortForward>> getPortForwarding(
            final Map<String, String> nodeService, final Predicate<String> select, final PortForward portForward) {
        return nodeService.entrySet().stream()
                .filter(e -> select.test(e.getKey()))
                .collect(Collectors.toMap(
                        Map.Entry::getKey,
                        e -> portForward.create(kubernetesClient
                                .services()
                                .inNamespace(namespace)
                                .withName(e.getValue()))));
    }

    /**
     * Creates a Hedera SDK client for direct service communication.
     *
     * @param nodeAddressBook The NodeAddressBook instance containing network details.
     * @return A Hedera SDK Client instance
     * @throws Exception if there is an error in generating the client.
     */
    Client getClient(NodeAddressBook nodeAddressBook) throws Exception {
        return Client.forNetwork(Collections.emptyMap()).setNetworkFromAddressBook(nodeAddressBook);
    }

    /**
     * Creates a Hedera SDK client context for direct or HAProxy service communication.
     *
     * @param certificates A map of certificate data indexed by node names.
     * @param pubKeys A map of public keys indexed by node names.
     * @param services A map of local port forwards indexed by service names.
     * @return A ClientContext instance for Hedera SDK client setup
     * @throws Exception if there is an error in generating the NodeAddressBook or client.
     */
    ClientContext getClient(
            final Map<String, byte[]> certificates,
            final Map<String, String> pubKeys,
            final Map<String, List<LocalPortForward>> services)
            throws Exception {
        return new ClientContext(
                getClient(nodeAddressBook(certificates, pubKeys, services)),
                services.values().stream().flatMap(Collection::stream).toList());
    }

    /**
     * Creates a NodeAddressBook instance from Kubernetes configuration data.
     *
     * @param certificates A map of certificate data indexed by node names.
     * @param pubKeys A map of public keys indexed by node names.
     * @param services A map of local port forwards indexed by service names.
     * @return A NodeAddressBook instance containing network details
     * @throws Exception if there is an error in generating the NodeAddressBook.
     */
    NodeAddressBook nodeAddressBook(
            final Map<String, byte[]> certificates,
            final Map<String, String> pubKeys,
            final Map<String, List<LocalPortForward>> services)
            throws Exception {
        final List<com.hedera.hashgraph.sdk.proto.NodeAddress> nodeAddresses = Arrays.stream(
                        configText().split("\n"))
                .filter(line -> line.startsWith("address"))
                .map(line -> line.split("\s*,\s*"))
                .filter(cfg -> services.containsKey(cfg[2]))
                .map(cfg -> nodeAddress(
                        cfg[1], cfg[9], services.get(cfg[2]), pubKeys.get(cfg[2]), certificates.get(cfg[2])))
                .toList();

        return NodeAddressBook.fromBytes(com.hedera.hashgraph.sdk.proto.NodeAddressBook.newBuilder()
                .addAllNodeAddress(nodeAddresses)
                .build()
                .toByteString());
    }

    /**
     * Retrieves service names indexed by node names from Kubernetes cluster based on a label selector.
     *
     * @param labelSelector The label selector to filter services.
     * @return A map of service names indexed by node names
     */
    Map<String, String> services(final String labelSelector) {
        return kubernetesClient
                .services()
                .inNamespace(namespace)
                .withLabelSelector(labelSelector)
                .list()
                .getItems()
                .stream()
                .collect(Collectors.toMap(
                        service -> service.getSpec().getSelector().get(Constants.LABEL_NODE_NAME),
                        service -> service.getMetadata().getName()));
    }

    /**
     * Retrieves service names indexed by node names for consensus nodes from Kubernetes cluster.
     *
     * @return A map of service names indexed by node names
     */
    Map<String, String> direct() {
        return services(LABEL_SELECTOR_CONSENSUS_NODE_APP);
    }

    /**
     * Retrieves service names indexed by node names for HAProxy nodes from Kubernetes cluster.
     *
     * @return A map of service names indexed by node names
     */
    Map<String, String> haproxy() {
        return services(LABEL_SELECTOR_HAPROXY_APP);
    }

    /**
     * Retrieves the configuration text from a Kubernetes config map named "cluster-config".
     *
     * @return The configuration text as a string
     */
    String configText() {
        return kubernetesClient
                .configMaps()
                .inNamespace(namespace)
                .withName("cluster-config")
                .get()
                .getData()
                .get("config.txt");
    }

    /**
     * Retrieves certificate data indexed by node names from Kubernetes secrets.
     *
     * @return A map of certificate data indexed by node names
     */
    Map<String, byte[]> certificates() {
        Base64.Decoder decoder = Base64.getDecoder();
        return kubernetesClient.secrets().inNamespace(namespace).list().getItems().stream()
                .filter(secret -> secret.getMetadata().getLabels().containsKey(Constants.LABEL_NODE_NAME))
                .filter(secret -> secret.getData()
                        .containsKey(String.format(
                                "hedera-%s.crt",
                                secret.getMetadata().getLabels().get(Constants.LABEL_NODE_NAME))))
                .collect(Collectors.toMap(
                        secret -> secret.getMetadata().getLabels().get(Constants.LABEL_NODE_NAME),
                        secret -> decoder.decode(secret.getData()
                                .get(String.format(
                                        "hedera-%s.crt",
                                        secret.getMetadata().getLabels().get(Constants.LABEL_NODE_NAME))))));
    }

    /**
     * Retrieves public keys indexed by node names from a Kubernetes config map named "public-keys".
     *
     * @return A map of public keys indexed by node names
     */
    Map<String, String> pubKeys() {
        return kubernetesClient
                .configMaps()
                .inNamespace(namespace)
                .withName("public-keys")
                .get()
                .getData()
                .entrySet()
                .stream()
                .filter(e -> e.getKey().startsWith("s-public-"))
                .collect(
                        Collectors.toMap(e -> e.getKey().substring(9, e.getKey().length() - 4), Map.Entry::getValue));
    }

    /**
     * Creates a NodeAddress for Hedera network communication based on provided parameters.
     *
     * @param nodeId The ID of the node.
     * @param accountId The account ID associated with the node.
     * @param portForwards A list of local port forwards for the service endpoints.
     * @param pubKey The public key of the node.
     * @param cert The certificate data for the node.
     * @return A NodeAddress instance containing network details
     * @throws RuntimeException if there is an error in generating the certificate or public key.
     */
    com.hedera.hashgraph.sdk.proto.NodeAddress nodeAddress(
            final String nodeId,
            final String accountId,
            final List<LocalPortForward> portForwards,
            final String pubKey,
            final byte[] cert) {
        try {
            Objects.requireNonNull(nodeId);
            Objects.requireNonNull(accountId);
            Objects.requireNonNull(portForwards);
            Objects.requireNonNull(pubKey);
            Objects.requireNonNull(cert);

            String[] account = accountId.split("\\.");
            String certHash = HexFormat.of()
                    .formatHex(MessageDigest.getInstance("SHA-384").digest(cert));
            com.hedera.hashgraph.sdk.proto.NodeAddress.Builder builder =
                    com.hedera.hashgraph.sdk.proto.NodeAddress.newBuilder()
                            .setNodeCertHash(ByteString.copyFromUtf8(certHash))
                            .setRSAPubKey(pubKey(pubKey.getBytes(StandardCharsets.UTF_8)))
                            .setNodeAccountId(AccountID.newBuilder()
                                    .setShardNum(Long.parseLong(account[0]))
                                    .setRealmNum(Long.parseLong(account[1]))
                                    .setAccountNum(Long.parseLong(account[2]))
                                    .build())
                            .setNodeId(Long.parseLong(nodeId));
            for (LocalPortForward portForward : portForwards) {
                builder.addServiceEndpoint(ServiceEndpoint.newBuilder()
                        .setIpAddressV4(ByteString.copyFrom(
                                portForward.getLocalAddress().getAddress()))
                        .setPort(portForward.getLocalPort())
                        .build());
            }
            return builder.build();
        } catch (GeneralSecurityException exception) {
            throw new RuntimeException(exception);
        }
    }

    /**
     * Converts a public key byte array to a hexadecimal string representation.
     *
     * @param bytes The byte array representing the public key.
     * @return A hexadecimal string representation of the public key
     * @throws GeneralSecurityException if there is an error in generating the certificate.
     */
    String pubKey(final byte[] bytes) throws GeneralSecurityException {
        return HexFormat.of()
                .formatHex(CertificateFactory.getInstance("X.509")
                        .generateCertificate(new ByteArrayInputStream(bytes))
                        .getPublicKey()
                        .getEncoded());
    }

    /** Context class for Hedera SDK client, implementing AutoCloseable to manage resources automatically. */
    public static class ClientContext implements AutoCloseable {
        public final Client client;
        private final Collection<LocalPortForward> portForwards;

        /**
         * Constructor to initialize the context with a Hedera SDK client and port forwards.
         *
         * @param client The Hedera SDK client instance.
         * @param portForwards A collection of local port forwards.
         */
        public ClientContext(final Client client, final Collection<LocalPortForward> portForwards) {
            this.client = client;
            this.portForwards = portForwards;
        }

        @Override
        public void close() {
            for (LocalPortForward portForward : portForwards) {
                try {
                    portForward.close();
                } catch (IOException ignored) {
                    // Ignored as the port forward may not have been fully initialized or connected
                }
            }
            try {
                client.close();
            } catch (TimeoutException ignored) {
                // Ignored as the client may not have been fully initialized or connected
            }
        }
    }
}
