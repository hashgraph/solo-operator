package com.hedera.solo.operator.test;

import static java.nio.charset.StandardCharsets.UTF_8;

import com.google.protobuf.ByteString;
import java.io.IOException;
import java.net.Socket;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.cert.X509Certificate;
import java.util.Base64;
import java.util.HexFormat;
import java.util.Objects;
import java.util.stream.Collectors;
import java.util.stream.IntStream;
import java.util.stream.Stream;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLSocket;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509TrustManager;
import org.junit.jupiter.api.Test;

public class CertDownloadTest {
    final static String BEGIN_CERTIFICATE = "-----BEGIN CERTIFICATE-----";
    final static String END_CERTIFICATE = "-----END CERTIFICATE-----";

    static String cert(final byte[] data) {
        StringBuilder sb = new StringBuilder();
        sb.append(BEGIN_CERTIFICATE).append("\n");
        sb.append(new String(Base64.getMimeEncoder(64, new byte[]{'\n'}).encode(data))).append("\n");
        sb.append(END_CERTIFICATE).append("\n");
        return sb.toString();
    }

//    static class SaveLastServerCertTrustManager implements X509TrustManager {
//        X509Certificate[] chain;
//
//        @Override
//        public void checkClientTrusted(X509Certificate[] chain, String authType) {}
//
//        @Override
//        public void checkServerTrusted(X509Certificate[] chain, String authType) {
//            this.chain = chain;
//        }
//
//        @Override
//        public X509Certificate[] getAcceptedIssuers() {
//            return new X509Certificate[0];
//        }
//    }

    static class ServiceEndpoint {
        String domainName;
        ByteString ipv4;
        int port;

        public ServiceEndpoint(ByteString ipv4, int port) {
            this.ipv4 = ipv4;
            this.port = port;
        }
    }
    @Test
    void clientTest() throws Exception {
        Stream.of(
//                ServiceEndpoint.newBuilder()
//                        .setDomainName("www.google.com")
//                        .setPort(443)
//                        .build(),
                new ServiceEndpoint(ByteString.copyFrom(new byte[] {10, 89, 0, (byte) 205}), 50211),
                        new ServiceEndpoint(ByteString.copyFrom(new byte[] {10, 89, 0, (byte) 205}), 50212))
                .map(CertDownloadTest::certHash)
                .filter(Objects::nonNull)
                .findFirst()
                .ifPresent(hash -> System.out.println("Certificate hash: " + hash));
    }

    static String certHash(ServiceEndpoint endpoint) {
        try {
            return hash(cert(host(endpoint), endpoint.port));
        } catch (Exception ignored) {
            return null;
        }
    }

//    String certHash(List<ServiceEndpoint> serviceEndpoints) throws Exception {
//        for (ServiceEndpoint endpoint : serviceEndpoints) {
//            try {
//                if (endpoint.getIpAddressV4().isEmpty()) {
//                    return certHash(endpoint.getDomainName(), endpoint.getPort());
//                }
//                String ip = IntStream.range(0, 4).mapToObj(endpoint.getIpAddressV4()::byteAt).map(Byte::toUnsignedInt).map(Objects::toString).collect(Collectors.joining("."));
//                return certHash(ip, endpoint.getPort());
//            } catch (Exception ignored) {}
//        }
//        return null;
//    }

    static String host(final ServiceEndpoint endpoint) {
        String host = endpoint.domainName;
        if (null == host || host.isEmpty()) {
            host = IntStream.range(0, 4)
                    .mapToObj(endpoint.ipv4::byteAt)
                    .map(Byte::toUnsignedInt)
                    .map(Objects::toString)
                    .collect(Collectors.joining("."));
        }
        return host;
    }

    static String hash(final String cert) throws NoSuchAlgorithmException {
        MessageDigest digest = MessageDigest.getInstance("SHA-384");
        HexFormat hex = HexFormat.of();
        return hex.formatHex(digest.digest(cert.getBytes(UTF_8)));
    }

    static String cert(final String host, int port) throws GeneralSecurityException, IOException {
        final X509Certificate[][] chain_ = new X509Certificate[1][0];
        X509TrustManager trustManager = new X509TrustManager() {
            @Override
            public void checkClientTrusted(X509Certificate[] chain, String authType) {}

            @Override
            public void checkServerTrusted(X509Certificate[] chain, String authType) {
                chain_[0] = chain;
            }

            @Override
            public X509Certificate[] getAcceptedIssuers() {
                return new X509Certificate[0];
            }
        };
        SSLContext sslContext = SSLContext.getInstance("TLS");
        sslContext.init(null, new TrustManager[]{trustManager}, null);
        SSLSocketFactory sslSocketFactory = sslContext.getSocketFactory();
        try (Socket socket = sslSocketFactory.createSocket(host, port)) {
            if (socket instanceof SSLSocket sslSocket) {
                sslSocket.startHandshake();
            }
        }
        return cert(chain_[0][0].getEncoded());
    }
}
