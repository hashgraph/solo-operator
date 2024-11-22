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
import org.awaitility.core.ConditionTimeoutException;

public class HederaSdkCheckAddressBook {

    public static List<NodeAddress> addresses(final Client client) throws Exception {
        final ByteString raw =
                new FileContentsQuery().setFileId(FileId.ADDRESS_BOOK).execute(client);
        assertThat(raw).isNotNull();
        return NodeAddressBook.fromBytes(raw).getNodeAddresses();
    }

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
            ConditionTimeoutException timeout = null;
            for (int attempt = 0; attempt < 10; attempt++) {
                assertion = null;
                timeout = null;
                try {
                    checkAddressBook(
                            sdkClientFactory, hederaSdkClientContextFactory, nodeName, expectedAddresses, false);
                    checkAddressBook(
                            sdkClientFactory, hederaSdkClientContextFactory, nodeName, expectedAddresses, true);
                } catch (AssertionError e) {
                    assertion = e;
                } catch (ConditionTimeoutException e) {
                    timeout = e;
                }
                if (assertion == null && timeout == null) {
                    break;
                }
            }
            if (assertion != null) {
                throw assertion;
            }
            if (timeout != null) {
                throw timeout;
            }
        }
    }
}
