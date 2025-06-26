// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.hederaclient;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.google.protobuf.ByteString;
import com.hedera.hashgraph.sdk.AccountId;
import com.hedera.hashgraph.sdk.Client;
import com.hedera.hashgraph.sdk.FileContentsQuery;
import com.hedera.hashgraph.sdk.FileId;
import com.hedera.hashgraph.sdk.NodeAddress;
import com.hedera.hashgraph.sdk.NodeAddressBook;
import com.hedera.hashgraph.sdk.PrivateKey;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.util.List;
import java.util.concurrent.TimeUnit;

/** This class contains utility methods to check the address book using Hedera SDK. */
public class HederaSdkCheckAddressBook {

    /**
     * Retrieves the addresses from the Hedera network's address book file.
     *
     * @param client The Hedera Client object used to connect and interact with the Hedera network.
     * @return A list of NodeAddress objects representing the nodes in the address book
     * @throws Exception If there is an error retrieving the file contents or parsing the address book.
     */
    public static List<NodeAddress> addresses(final Client client) throws Exception {
        final ByteString raw =
                new FileContentsQuery().setFileId(FileId.ADDRESS_BOOK).execute(client);
        assertThat(raw).isNotNull();
        return NodeAddressBook.fromBytes(raw).getNodeAddresses();
    }

    /**
     * Checks the address book for a given node in both secure and insecure modes.
     *
     * @param sdkClientFactory The factory to create Hedera SDK client context.
     * @param hederaSdkClientContextFactory The factory to create Hedera SDK client context for the node.
     * @param nodeName The name of the consensus node whose address book is to be checked.
     * @param expectedAddresses The number of expected addresses in the address book.
     * @param secure Whether to check the address book in secure mode (true) or insecure mode (false).
     */
    public static void checkAddressBook(
            final HederaSdkClientFactory sdkClientFactory,
            final HederaSdkClientContextFactory hederaSdkClientContextFactory,
            final String nodeName,
            final int expectedAddresses,
            final boolean secure) {
        try (HederaSdkClientFactory.ClientContext context =
                hederaSdkClientContextFactory.create(sdkClientFactory, nodeName, secure)) {
            await().atMost(15, TimeUnit.SECONDS).untilAsserted(() -> {
                Client client = context.client;

                client.setOperator(
                        AccountId.fromString(HederaSdkClientFactory.DEFAULT_ACCOUNT_ID),
                        PrivateKey.fromString(HederaSdkClientFactory.DEFAULT_PRIVATE_KEY));

                if (secure) {
                    client.setTransportSecurity(true).setVerifyCertificates(true);
                    assertThat(addresses(client).size()).isEqualTo(expectedAddresses);
                } else {
                    client.setTransportSecurity(false);
                    assertThat(addresses(client).size()).isEqualTo(expectedAddresses);
                }
            });
        }
    }

    /**
     * Checks the address book for multiple consensus nodes in both secure and insecure modes.
     *
     * @param hederaSdkClientContextFactory The factory to create Hedera SDK client context for the node.
     * @param kubernetesClient The Kubernetes client used to interact with the Kubernetes cluster.
     * @param namespace The Kubernetes namespace where the consensus nodes are deployed.
     * @param expectedAddresses The number of expected addresses in the address book for each node.
     * @param consensusNodeNames An array of names of the consensus nodes whose address books are to be checked.
     * @throws AssertionError If there is an assertion error during checking.
     * @throws Exception If there is an error retrieving the file contents or parsing the address book.
     */
    public static void checkAddressBook(
            final HederaSdkClientContextFactory hederaSdkClientContextFactory,
            final KubernetesClient kubernetesClient,
            final String namespace,
            final int expectedAddresses,
            final String... consensusNodeNames)
            throws AssertionError, Exception {
        HederaSdkClientFactory sdkClientFactory = new HederaSdkClientFactory(kubernetesClient, namespace);
        for (String nodeName : consensusNodeNames) {
            AssertionError assertion = null;
            Exception exception = null;
            for (int attempt = 0; attempt < 10; attempt++) {
                assertion = null;
                exception = null;
                try {
                    checkAddressBook(
                            sdkClientFactory, hederaSdkClientContextFactory, nodeName, expectedAddresses, false);
                    checkAddressBook(
                            sdkClientFactory, hederaSdkClientContextFactory, nodeName, expectedAddresses, true);
                } catch (AssertionError e) {
                    assertion = e;
                } catch (Exception e) {
                    exception = e;
                }
                if (assertion == null && exception == null) {
                    break;
                }
            }
            if (assertion != null) {
                throw assertion;
            }
            if (exception != null) {
                throw exception;
            }
        }
    }
}
