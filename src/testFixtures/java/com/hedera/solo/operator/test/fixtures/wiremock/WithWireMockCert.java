// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.wiremock;

import io.smallrye.certs.Format;
import io.smallrye.certs.junit5.Certificate;
import io.smallrye.certs.junit5.Certificates;
import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;
import org.junit.jupiter.api.extension.ExtendWith;

@Target({ElementType.TYPE})
@Retention(RetentionPolicy.RUNTIME)
@Certificates(
        baseDir = WithWireMockCert.WM_KS_DIR,
        certificates =
                @Certificate(
                        duration = 99999,
                        formats = {Format.JKS},
                        name = WithWireMockCert.WM_KS_NAME,
                        password = WithWireMockCert.WM_KS_PASS))
@ExtendWith(WireMockTrustStoreExtension.class)
public @interface WithWireMockCert {
    String WM_KS_DIR = "build/generated/smallrye-certificate-generator";
    String WM_KS_NAME = "wiremock";
    String WM_KS_FILE = WM_KS_DIR + "/" + WM_KS_NAME + "-keystore.jks";
    String WM_KS_PASS = "password";
    String WM_TS_FILE = WM_KS_DIR + "/" + WM_KS_NAME + "-truststore.jks";
    String WM_TS_PASS = WM_KS_PASS;
}
