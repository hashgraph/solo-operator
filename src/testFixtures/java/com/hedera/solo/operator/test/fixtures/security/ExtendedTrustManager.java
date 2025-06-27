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

/**
 * ExtendedTrustManager is a custom trust manager that extends the functionality of an X509TrustManager. It allows for
 * adding and removing trusted certificates dynamically, including from file sources.
 */
public class ExtendedTrustManager implements X509TrustManager {
    private static final ExtendedTrustManager self = new ExtendedTrustManager();
    private final MapCounter<byte[], X509TrustManager> map = new MapCounter<>(ConcurrentHashMap::new);
    private final X509TrustManager defaultTrustManager;
    private SSLContext defaultContext;

    /**
     * Private constructor to initialize the ExtendedTrustManager with a default trust manager.
     *
     * @throws RuntimeException If there is an issue with initializing the default trust manager.
     */
    private ExtendedTrustManager() {
        try {
            defaultTrustManager = trustManager(null);
        } catch (NoSuchAlgorithmException | KeyStoreException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * Adds a trusted certificate source from a file.
     *
     * @param file The path to the file containing the trusted certificates.
     * @param password The password for accessing the KeyStore in the file.
     * @return A Resource object that encapsulates the added trust source
     * @throws IOException If there is an issue with reading the file.
     * @throws NoSuchAlgorithmException If the SHA-256 algorithm is not available.
     * @throws CertificateException If there is a problem with certificate processing.
     * @throws KeyStoreException If there is a problem with the KeyStore.
     * @throws KeyManagementException If there is an issue with SSL context management.
     */
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

    /**
     * Gets a collection of all trust managers, including the default and added ones.
     *
     * @return A list of X509TrustManager objects
     */
    private Collection<X509TrustManager> trustManagers() {
        return Stream.concat(Stream.of(defaultTrustManager), map.values().stream())
                .toList();
    }

    /** An interface for inspecting and checking trust managers. */
    private interface Inspector {
        /**
         * Checks the provided trust manager.
         *
         * @param trustManager The X509TrustManager to be checked
         * @throws CertificateException If the trust manager does not trust the certificate
         */
        void check(final X509TrustManager trustManager) throws CertificateException;
    }

    /**
     * Iterates over all trust managers to perform the necessary checks.
     *
     * @param inspector The inspection function.
     * @throws CertificateException If any of the checks fail.
     */
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

    /**
     * Adds a trusted certificate source from a file to the map.
     *
     * @param file The path to the file containing the trusted certificates.
     * @param password The password for accessing the KeyStore in the file.
     * @return A Resource object that encapsulates the added trust source
     * @throws IOException If there is an issue with reading the file.
     * @throws NoSuchAlgorithmException If the SHA-256 algorithm is not available.
     * @throws CertificateException If there is a problem with certificate processing.
     * @throws KeyStoreException If there is a problem with the KeyStore.
     * @throws KeyManagementException If there is an issue with SSL context management.
     */
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

    /**
     * Removes a trusted certificate source from the map based on its digest.
     *
     * @param digest The digest of the trust source to be removed.
     */
    private void remove(final Object digest) {
        if (digest instanceof byte[] bytes) {
            synchronized (this) {
                if (map.decrementAndRemoveIfNone(bytes) != null && map.isEmpty()) {
                    deactivate();
                }
            }
        }
    }

    /**
     * Activates the trust managers and sets them as the default SSLContext.
     *
     * @throws NoSuchAlgorithmException If the TLS algorithm is not available.
     * @throws KeyStoreException If there is a problem with the KeyStore.
     * @throws KeyManagementException If there is an issue with SSL context management.
     */
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

    /** Deactivates the current trust managers and resets the default SSLContext to the original one. */
    private void deactivate() {
        SSLContext.setDefault(defaultContext);
        defaultContext = null;
    }

    /**
     * Creates a KeyStore from a file and extracts its trusted certificates, returning their digest and the trust
     * manager.
     *
     * @param file The path to the file containing the KeyStore.
     * @param password The password for accessing the KeyStore in the file.
     * @return A map entry with the digest of the file content and the corresponding X509TrustManager
     * @throws KeyStoreException If there is a problem with the KeyStore.
     * @throws IOException If there is an issue with reading the file.
     * @throws CertificateException If there is a problem with certificate processing.
     * @throws NoSuchAlgorithmException If the SHA-256 algorithm is not available.
     */
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

    /**
     * Creates a X509TrustManager from a KeyStore.
     *
     * @param keyStore The KeyStore containing the trusted certificates.
     * @return An X509TrustManager instance
     * @throws NoSuchAlgorithmException If the default algorithm for trust managers is not available.
     * @throws KeyStoreException If there is a problem with the KeyStore.
     */
    private X509TrustManager trustManager(final KeyStore keyStore) throws NoSuchAlgorithmException, KeyStoreException {
        TrustManagerFactory factory = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        factory.init(keyStore);
        return Arrays.stream(factory.getTrustManagers())
                .filter(X509TrustManager.class::isInstance)
                .findFirst()
                .map(X509TrustManager.class::cast)
                .orElseThrow();
    }

    /** Resource class to encapsulate and manage the added trust sources, providing a close method for cleanup. */
    public class Resource implements AutoCloseable {
        private Object object;

        /**
         * Private constructor to initialize the Resource with an object.
         *
         * @param object The object to be encapsulated.
         */
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
