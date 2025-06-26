// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.wiremock;

import static com.hedera.solo.operator.test.fixtures.security.ExtendedTrustManager.extendWithTrustSource;
import static com.hedera.solo.operator.test.fixtures.wiremock.WithWireMockCert.WM_TS_FILE;
import static com.hedera.solo.operator.test.fixtures.wiremock.WithWireMockCert.WM_TS_PASS;

import com.hedera.solo.operator.test.fixtures.security.ExtendedTrustManager;
import java.nio.file.Path;
import org.junit.jupiter.api.extension.AfterAllCallback;
import org.junit.jupiter.api.extension.BeforeAllCallback;
import org.junit.jupiter.api.extension.ExtensionContext;
import org.junit.jupiter.api.extension.ParameterContext;
import org.junit.jupiter.api.extension.ParameterResolutionException;
import org.junit.jupiter.api.extension.ParameterResolver;

/**
 * The WireMockTrustStoreExtension class is a custom JUnit Jupiter extension that provides trust store configuration for
 * WireMock server.
 */
public class WireMockTrustStoreExtension implements AfterAllCallback, BeforeAllCallback, ParameterResolver {
    private ExtendedTrustManager.Resource resource;

    @Override
    public void afterAll(final ExtensionContext context) throws Exception {
        resource.close();
    }

    @Override
    public void beforeAll(final ExtensionContext context) throws Exception {
        resource = extendWithTrustSource(Path.of(WM_TS_FILE), WM_TS_PASS);
    }

    @Override
    public boolean supportsParameter(final ParameterContext parameterContext, final ExtensionContext extensionContext)
            throws ParameterResolutionException {
        return parameterContext.getDeclaringExecutable().getDeclaringClass().isAnnotationPresent(WithWireMockCert.class)
                && parameterContext.getParameter().getType().isAssignableFrom(WireMockServerFactory.class);
    }

    @Override
    public Object resolveParameter(final ParameterContext parameterContext, final ExtensionContext extensionContext)
            throws ParameterResolutionException {
        return new WireMockServerFactory();
    }
}
