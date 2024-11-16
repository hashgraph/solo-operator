// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.hederaclient;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_HAPROXY;

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

public class HederaSdkClientFactory {
    public static final String DEFAULT_ACCOUNT_ID = "0.0.2";
    public static final String DEFAULT_PRIVATE_KEY =
            "302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137";
    private final KubernetesClient kubernetesClient;
    private final String namespace;

    public HederaSdkClientFactory(final KubernetesClient kubernetesClient, final String namespace) throws Exception {
        this.kubernetesClient = kubernetesClient;
        this.namespace = namespace;
    }

    public interface PortForward {
        List<LocalPortForward> create(final PortForwardable portForwardable);
    }

    public ClientContext directContextCreator(final String consensusNode, boolean secure) {
        return createClientContext(direct(), consensusNode, secure);
    }

    public ClientContext haproxyContextCreator(final String consensusNode, boolean secure) {
        return createClientContext(haproxy(), consensusNode, secure);
    }

    ClientContext createClientContext(
            final Map<String, String> nodeService, final String consensusNode, boolean secure) {
        return getClientContext(
                nodeService, consensusNode::equals, resource -> (secure ? Stream.of(50212) : Stream.of(50211))
                        .map(resource::portForward)
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

    Client getClient(NodeAddressBook nodeAddressBook) throws Exception {
        return Client.forNetwork(Collections.emptyMap()).setNetworkFromAddressBook(nodeAddressBook, true);
    }

    ClientContext getClient(final Map<String, List<LocalPortForward>> services) throws Exception {
        return new ClientContext(
                getClient(nodeAddressBook(services)),
                services.values().stream().flatMap(Collection::stream).toList());
    }

    NodeAddressBook nodeAddressBook(final Map<String, List<LocalPortForward>> services) throws Exception {
        Map<String, byte[]> certificates = certificates();
        Map<String, String> pubKeys = pubKeys();
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

    String pubKey(final byte[] bytes) throws GeneralSecurityException {
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
        public void close() {
            for (LocalPortForward portForward : portForwards) {
                try {
                    portForward.close();
                } catch (IOException ignored) {
                }
            }
            try {
                client.close();
            } catch (TimeoutException ignored) {
            }
        }
    }
}
