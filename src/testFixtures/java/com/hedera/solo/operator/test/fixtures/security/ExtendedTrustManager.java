// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.security;

import com.hedera.solo.operator.library.data.Digest;
import java.io.IOException;
import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.DigestInputStream;
import java.security.KeyManagementException;
import java.security.KeyStore;
import java.security.KeyStoreException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.cert.CertificateException;
import java.security.cert.X509Certificate;
import java.util.Arrays;
import java.util.Collection;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.stream.Stream;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManager;
import javax.net.ssl.TrustManagerFactory;
import javax.net.ssl.X509TrustManager;

public class ExtendedTrustManager implements X509TrustManager {
    private static final ExtendedTrustManager self = new ExtendedTrustManager();
    private final MapCounter<byte[], X509TrustManager> map = new MapCounter<>(ConcurrentHashMap::new);
    private final X509TrustManager defaultTrustManager;
    private SSLContext defaultContext;

    private ExtendedTrustManager() {
        try {
            defaultTrustManager = trustManager(null);
        } catch (NoSuchAlgorithmException | KeyStoreException e) {
            throw new RuntimeException(e);
        }
    }

    public static Resource extendWithTrustSource(final Path file, final String password)
            throws IOException, CertificateException, NoSuchAlgorithmException, KeyStoreException,
                    KeyManagementException {
        return self.add(file, password);
    }

    @Override
    public void checkClientTrusted(final X509Certificate[] chain, final String authType) throws CertificateException {
        forEach(tm -> tm.checkClientTrusted(chain, authType));
    }

    @Override
    public void checkServerTrusted(final X509Certificate[] chain, final String authType) throws CertificateException {
        forEach(tm -> tm.checkServerTrusted(chain, authType));
    }

    @Override
    public X509Certificate[] getAcceptedIssuers() {
        return trustManagers().stream()
                .flatMap(tm -> Arrays.stream(tm.getAcceptedIssuers()))
                .toArray(X509Certificate[]::new);
    }

    private Collection<X509TrustManager> trustManagers() {
        return Stream.concat(Stream.of(defaultTrustManager), map.values().stream())
                .toList();
    }

    private interface Inspector {
        void check(final X509TrustManager trustManager) throws CertificateException;
    }

    private void forEach(final Inspector inspector) throws CertificateException {
        CertificateException exception = null;
        for (X509TrustManager trustManager : trustManagers()) {
            try {
                inspector.check(trustManager);
                return;
            } catch (CertificateException e) {
                if (exception == null) {
                    exception = e;
                } else {
                    exception.addSuppressed(e);
                }
            }
        }
        throw exception == null ? new CertificateException("No trust manager found") : exception;
    }

    private Resource add(final Path file, final String password)
            throws IOException, NoSuchAlgorithmException, CertificateException, KeyStoreException,
                    KeyManagementException {
        Map.Entry<byte[], X509TrustManager> entry = trustManager(file, password);
        synchronized (this) {
            boolean empty = map.isEmpty();
            map.putIfAbsentAndIncrement(entry.getKey(), entry.getValue());
            if (empty) {
                activate();
            }
        }
        return new Resource(entry.getKey());
    }

    private void remove(final Object digest) {
        if (digest instanceof byte[] bytes) {
            synchronized (this) {
                if (map.decrementAndRemoveIfNone(bytes) != null && map.isEmpty()) {
                    deactivate();
                }
            }
        }
    }

    private void activate() throws NoSuchAlgorithmException, KeyStoreException, KeyManagementException {
        TrustManagerFactory factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        factory.init((KeyStore) null);
        TrustManager[] trustManagers = Arrays.stream(factory.getTrustManagers())
                .map(tm -> tm instanceof X509TrustManager ? this : tm)
                .toArray(TrustManager[]::new);
        SSLContext context = SSLContext.getInstance("TLS");
        context.init(null, trustManagers, null);
        defaultContext = SSLContext.getDefault();
        SSLContext.setDefault(context);
    }

    private void deactivate() {
        SSLContext.setDefault(defaultContext);
        defaultContext = null;
    }

    private Map.Entry<byte[], X509TrustManager> trustManager(final Path file, final String password)
            throws KeyStoreException, IOException, CertificateException, NoSuchAlgorithmException {
        KeyStore keyStore = KeyStore.getInstance(KeyStore.getDefaultType());
        MessageDigest digest = Digest.SHA_256.instance();
        digest.update(password.getBytes());
        try (InputStream inputStream = Files.newInputStream(file);
                DigestInputStream digestInputStream = new DigestInputStream(inputStream, digest)) {
            keyStore.load(digestInputStream, password.toCharArray());
            return Map.entry(digest.digest(), trustManager(keyStore));
        }
    }

    private X509TrustManager trustManager(final KeyStore keyStore) throws NoSuchAlgorithmException, KeyStoreException {
        TrustManagerFactory factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        factory.init(keyStore);
        return Arrays.stream(factory.getTrustManagers())
                .filter(X509TrustManager.class::isInstance)
                .findFirst()
                .map(X509TrustManager.class::cast)
                .orElseThrow();
    }

    public class Resource implements AutoCloseable {
        private Object object;

        private Resource(final Object object) {
            this.object = object;
        }

        @Override
        public void close() {
            if (object != null) {
                remove(object);
                object = null;
            }
        }
    }
}
