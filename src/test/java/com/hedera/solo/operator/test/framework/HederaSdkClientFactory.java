/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.test.framework;

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_CONSENSUS_CAPSULE_NAME_KEY;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY;
import static java.util.concurrent.TimeUnit.SECONDS;

import com.google.protobuf.ByteString;
import com.hedera.hashgraph.sdk.AccountId;
import com.hedera.hashgraph.sdk.Client;
import com.hedera.hashgraph.sdk.NodeAddress;
import com.hedera.hashgraph.sdk.NodeAddressBook;
import com.hedera.hashgraph.sdk.proto.AccountID;
import com.hedera.hashgraph.sdk.proto.ServiceEndpoint;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.LocalPortForward;
import io.fabric8.kubernetes.client.dsl.PortForwardable;
import java.io.ByteArrayInputStream;
import java.io.File;
import java.io.IOException;
import java.lang.invoke.MethodHandles;
import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.security.MessageDigest;
import java.security.cert.CertificateFactory;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.Collection;
import java.util.Collections;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeoutException;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class HederaSdkClientFactory {
    public static final String DEFAULT_ACCOUNT_ID = "0.0.2";
    public static final String DEFAULT_PRIVATE_KEY =
            "302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137";
    KubernetesClient kubernetesClient;
    String namespace;
    Workaround workaround = new Workaround();

    public HederaSdkClientFactory(final KubernetesClient kubernetesClient, final String namespace) throws Exception {
        this.kubernetesClient = kubernetesClient;
        this.namespace = namespace;
    }

    public interface PortForward {
        List<LocalPortForward> create(final PortForwardable portForwardable);
    }

    public ClientContext directContextCreator(final String consensusNode) {
        return createClientContext(direct(), consensusNode);
    }

    public ClientContext haproxyContextCreator(final String consensusNode) {
        return createClientContext(haproxy(), consensusNode);
    }

    private static final Logger log =
            LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    ClientContext createClientContext(final Map<String, String> nodeService, final String consensusNode) {
        try {
            ProcessBuilder pb = new ProcessBuilder("./netstat", "-nalp");
            File output = File.createTempFile(".netstat-", ".out");
            pb.redirectOutput(output);
            Process p = pb.start();
            p.waitFor(10, SECONDS);
            log.info("netstat: {}", Files.readString(output.toPath()));
        } catch (Exception e) {
            log.info("netstat: {}", e.getMessage());
        }
        List<LocalPortForward> portForwards = new ArrayList<>();
        try {
            return getClientContext(nodeService, consensusNode::equals, resource -> Stream.of(50211, 50212)
                    .map(port -> {
                        // include address here
                        LocalPortForward portForward = resource.portForward(port, port);
                        portForwards.add(portForward);
                        log.info("opened portForward: {}", portForward.getLocalPort());
                        return portForward;
                    })
                    .toList());
        } catch (Exception e) {
            for (LocalPortForward portForward : portForwards) {
                try {
                    portForward.close();
                } catch (IOException ex) {
                    throw new RuntimeException(ex);
                }
            }
            throw e;
        }
    }

    ClientContext getClientContext(
            final Map<String, String> nodeService, final Predicate<String> predicate, final PortForward portForward) {
        try {
            return getClient(getPortForwarding(nodeService, predicate, portForward));
        } catch (Exception exception) {
            throw new RuntimeException(exception);
        }
    }

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

    ClientContext getClient(final Map<String, List<LocalPortForward>> services) throws Exception {
        return new ClientContext(
                workaround.getClient(nodeAddressBook(services)),
                services.values().stream().flatMap(Collection::stream).toList());
    }

    NodeAddressBook nodeAddressBook(final Map<String, List<LocalPortForward>> services) throws Exception {
        Map<String, byte[]> certificates = certificates();
        Map<String, String> pubKeys = pubKeys();
        return NodeAddressBook.fromBytes(com.hedera.hashgraph.sdk.proto.NodeAddressBook.newBuilder()
                .addAllNodeAddress(Arrays.stream(configText().split("\n"))
                        .filter(line -> line.startsWith("address"))
                        .map(line -> line.split("\s*,\s*"))
                        .map(cfg -> nodeAddress(
                                cfg[1], cfg[9], services.get(cfg[2]), pubKeys.get(cfg[2]), certificates.get(cfg[2])))
                        .filter(Objects::nonNull)
                        .toList())
                .build()
                .toByteString());
    }

    Map<String, String> services(final String labelSelector) {
        return kubernetesClient
                .services()
                .inNamespace(namespace)
                .withLabelSelector(labelSelector)
                .list()
                .getItems()
                .stream()
                .collect(Collectors.toMap(
                        service -> service.getSpec().getSelector().get(LABEL_CONSENSUS_CAPSULE_NAME_KEY),
                        service -> service.getMetadata().getName()));
    }

    Map<String, String> direct() {
        return services(LABEL_SELECTOR_CONSENSUS_NODE);
    }

    Map<String, String> haproxy() {
        return services(LABEL_SELECTOR_HAPROXY);
    }

    String configText() {
        return kubernetesClient
                .configMaps()
                .inNamespace(namespace)
                .withName("cluster-config")
                .get()
                .getData()
                .get("config.txt");
    }

    Map<String, byte[]> certificates() {
        Base64.Decoder decoder = Base64.getDecoder();
        return kubernetesClient.secrets().inNamespace(namespace).list().getItems().stream()
                .filter(secret -> secret.getMetadata().getLabels().containsKey("consensus-capsule"))
                .filter(secret -> secret.getData().containsKey("hedera.crt"))
                .collect(Collectors.toMap(
                        secret -> secret.getMetadata().getLabels().get("consensus-capsule"),
                        secret -> decoder.decode(secret.getData().get("hedera.crt"))));
    }

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

    com.hedera.hashgraph.sdk.proto.NodeAddress nodeAddress(
            final String nodeId,
            final String accountId,
            final List<LocalPortForward> portForwards,
            final String pubKey,
            final byte[] cert) {
        try {
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
                        .setIpAddressV4(ByteString.copyFrom(new byte[] {127, 0, 0, 1})) // portForward.getLocalAddress()
                        .setPort(portForward.getLocalPort())
                        .build());
            }
            return builder.build();
        } catch (Exception e) {
            return null;
        }
    }

    String pubKey(final byte[] bytes) throws Exception {
        return HexFormat.of()
                .formatHex(CertificateFactory.getInstance("X.509")
                        .generateCertificate(new ByteArrayInputStream(bytes))
                        .getPublicKey()
                        .getEncoded());
    }

    public static class ClientContext implements AutoCloseable {
        private static final Logger log =
                LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
        public final Client client;
        private final Collection<LocalPortForward> portForwards;

        public ClientContext(final Client client, final Collection<LocalPortForward> portForwards) {
            this.client = client;
            this.portForwards = portForwards;
        }

        @Override
        public void close() throws Exception {
            List<Exception> exceptions = new ArrayList<>();
            try {
                client.close();
            } catch (TimeoutException e) {
                exceptions.add(e);
            }
            for (LocalPortForward portForward : portForwards) {
                try {
                    int port = portForward.getLocalPort();
                    portForward.close();
                    log.info("portForward closed: {}", port);
                } catch (IOException e) {
                    exceptions.add(e);
                }
            }
            if (!exceptions.isEmpty()) {
                Exception exception = exceptions.removeFirst();
                exceptions.forEach(exception::addSuppressed);
                throw exception;
            }
        }
    }

    /**
     * Workaround to configure client to use TLS with custom network.
     *
     * @see <a href="https://github.com/hashgraph/hedera-sdk-java/pull/1901">PR #1901</a>
     * @see <a href="https://github.com/hashgraph/hedera-sdk-java/pull/1930">PR #1930</a>
     */
    static class Workaround {
        Field clientNetworkField;
        Field networkAddressBookField;
        Field networkNodesField;
        Field nodeAccountIdField;
        Field nodeAddressBookEntry;

        Workaround() throws Exception {
            try (Client sdkClient = Client.forNetwork(Map.of("localhost:0", new AccountId(0)))) {
                clientNetworkField = Client.class.getDeclaredField("network");
                clientNetworkField.setAccessible(true);
                Object network = clientNetworkField.get(sdkClient);
                Class<?> networkClass = network.getClass();
                networkAddressBookField = networkClass.getDeclaredField("addressBook");
                networkAddressBookField.setAccessible(true);
                networkNodesField = networkClass.getSuperclass().getDeclaredField("nodes");
                networkNodesField.setAccessible(true);
                List<?> nodes = (List<?>) networkNodesField.get(network);
                Class<?> nodeClass = nodes.getFirst().getClass();
                nodeAccountIdField = nodeClass.getDeclaredField("accountId");
                nodeAccountIdField.setAccessible(true);
                nodeAddressBookEntry = nodeClass.getDeclaredField("addressBookEntry");
                nodeAddressBookEntry.setAccessible(true);
            }
        }

        Client getClient(final NodeAddressBook nodeAddressBook) throws Exception {
            return setNetworkFromAddressBook(Client.forNetwork(Collections.emptyMap()), nodeAddressBook);
        }

        void setAddressBook(final Client client, final Map<AccountId, NodeAddress> addressBook) throws Exception {
            networkAddressBookField.set(clientNetworkField.get(client), addressBook);
        }

        List<?> nodes(final Client client) throws Exception {
            return (List<?>) networkNodesField.get(clientNetworkField.get(client));
        }

        AccountId getAccountId(final Object node) throws Exception {
            return (AccountId) nodeAccountIdField.get(node);
        }

        void setAddressBookEntry(final Object node, final NodeAddress nodeAddress) throws Exception {
            nodeAddressBookEntry.set(node, nodeAddress);
        }

        Client setNetworkFromAddressBook(final Client client, final NodeAddressBook nodeAddressBook) throws Exception {
            client.setNetworkFromAddressBook(nodeAddressBook);
            Map<AccountId, NodeAddress> addressBook = nodeAddressBook.getNodeAddresses().stream()
                    .filter(nodeAddress -> Objects.nonNull(nodeAddress.getAccountId()))
                    .collect(Collectors.toMap(NodeAddress::getAccountId, Function.identity()));
            setAddressBook(client, addressBook);
            for (var node : nodes(client)) {
                setAddressBookEntry(node, addressBook.get(getAccountId(node)));
            }
            return client;
        }
    }
}
