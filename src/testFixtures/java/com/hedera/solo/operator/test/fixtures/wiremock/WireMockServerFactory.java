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
    protected WireMockServerFactory() {}

    public WireMockConfiguration config() {
        return WireMockConfiguration.wireMockConfig()
                .dynamicPort()
                .dynamicHttpsPort()
                .keystorePath(WM_KS_FILE)
                .keystorePassword(WM_KS_PASS);
    }

    public WireMockServer wm() {
        return wm(config());
    }

    public WireMockServer wm(final WireMockConfiguration config) {
        WireMockServer wm = new WireMockServer(config);
        wm.start();
        return wm;
    }
}
