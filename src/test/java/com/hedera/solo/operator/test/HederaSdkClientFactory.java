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

package com.hedera.solo.operator.test;

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
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
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
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import java.util.stream.Stream;

public class HederaSdkClientFactory {
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

    public Supplier<ClientContext> directContextCreator(final String consensusNode) {
        return createClientContext(direct(), consensusNode);
    }

    public Supplier<ClientContext> haproxyContextCreator(final String consensusNode) {
        return createClientContext(haproxy(), consensusNode);
    }

    Supplier<ClientContext> createClientContext(final Map<String, String> nodeService, final String consensusNode) {
        return () -> getClientContext(nodeService, consensusNode::equals, resource -> Stream.of(50211, 50212)
                .map(port -> resource.portForward(port, port))
                .toList());
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
        final List<com.hedera.hashgraph.sdk.proto.NodeAddress> nodeAddresses = Arrays.stream(
                        configText().split("\n"))
                .filter(line -> line.startsWith("address"))
                .map(line -> line.split("\s*,\s*"))
                .map(cfg -> nodeAddress(
                        cfg[1], cfg[9], services.get(cfg[2]), pubKeys.get(cfg[2]), certificates.get(cfg[2])))
                .filter(Objects::nonNull)
                .toList();

        return NodeAddressBook.fromBytes(com.hedera.hashgraph.sdk.proto.NodeAddressBook.newBuilder()
                .addAllNodeAddress(nodeAddresses)
                .build()
                .toByteString());
    }

    Map<String, String> services(final String labelSelector, final String selector) {
        return kubernetesClient
                .services()
                .inNamespace(namespace)
                .withLabelSelector(labelSelector)
                .list()
                .getItems()
                .stream()
                .collect(Collectors.toMap(
                        service -> service.getSpec().getSelector().get(selector),
                        service -> service.getMetadata().getName()));
    }

    Map<String, String> direct() {
        return services("app=consensus-node", "name");
    }

    Map<String, String> haproxy() {
        return services("app=haproxy", "consensus-capsule");
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
                .filter(secret -> secret.getData()
                        .containsKey(String.format(
                                "hedera-%s.crt",
                                secret.getMetadata().getLabels().get("consensus-capsule"))))
                .collect(Collectors.toMap(
                        secret -> secret.getMetadata().getLabels().get("consensus-capsule"),
                        secret -> decoder.decode(secret.getData()
                                .get(String.format(
                                        "hedera-%s.crt",
                                        secret.getMetadata().getLabels().get("consensus-capsule"))))));
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
                        .setIpAddressV4(ByteString.copyFrom(new byte[] {127, 0, 0, 1}))
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
        public final Client client;
        private final Collection<LocalPortForward> portForwards;

        public ClientContext(final Client client, final Collection<LocalPortForward> portForwards) {
            this.client = client;
            this.portForwards = portForwards;
        }

        @Override
        public void close() throws Exception {
            client.close();
            for (LocalPortForward portForward : portForwards) {
                portForward.close();
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
        Method networkSetNetwork;

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
                networkSetNetwork = networkClass.getSuperclass().getDeclaredMethod("setNetwork", Map.class);
                networkSetNetwork.setAccessible(true);
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

        void setNetwork(final Client client, final Map<String, AccountId> addressBook) throws Exception {
            networkSetNetwork.invoke(clientNetworkField.get(client), addressBook);
        }

        Client setNetworkFromAddressBook(final Client client, final NodeAddressBook nodeAddressBook) throws Exception {
            setNetwork(
                    client,
                    nodeAddressBook.getNodeAddresses().stream()
                            .flatMap(nodeAddress -> nodeAddress.getAddresses().stream()
                                    .map(endpoint -> Map.entry(
                                            endpoint.toString(), Objects.requireNonNull(nodeAddress.getAccountId()))))
                            .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue)));
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
