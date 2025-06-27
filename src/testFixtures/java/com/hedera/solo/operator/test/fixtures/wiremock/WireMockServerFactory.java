// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.wiremock;

import static com.hedera.solo.operator.test.fixtures.wiremock.WithWireMockCert.WM_KS_FILE;
import static com.hedera.solo.operator.test.fixtures.wiremock.WithWireMockCert.WM_KS_PASS;

import com.github.tomakehurst.wiremock.WireMockServer;
import com.github.tomakehurst.wiremock.core.WireMockConfiguration;

/**
 * Provider of {@code WireMockServer} and/or {@code WireMockConfiguration} to work with https.
 *
 * @see WithWireMockCert
 */
public class WireMockServerFactory {
    /** Default constructor is protected to prevent instantiation. */
    protected WireMockServerFactory() {}

    /**
     * Returns a default {@code WireMockConfiguration} object configured for dynamic ports and HTTPS.
     *
     * @return A {@code WireMockConfiguration} instance
     */
    public WireMockConfiguration config() {
        return WireMockConfiguration.wireMockConfig()
                .dynamicPort()
                .dynamicHttpsPort()
                .keystorePath(WM_KS_FILE)
                .keystorePassword(WM_KS_PASS);
    }

    /**
     * Creates and starts a {@code WireMockServer} using the default configuration.
     *
     * @return A started {@code WireMockServer}
     */
    public WireMockServer wm() {
        return wm(config());
    }

    /**
     * Creates and starts a {@code WireMockServer} with a custom {@code WireMockConfiguration}.
     *
     * @param config The configuration for the {@code WireMockServer}.
     * @return A started {@code WireMockServer}
     */
    public WireMockServer wm(final WireMockConfiguration config) {
        WireMockServer wm = new WireMockServer(config);
        wm.start();
        return wm;
    }
}
