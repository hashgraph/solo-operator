// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.data;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.NoSuchProviderException;
import java.util.HexFormat;
import java.util.stream.Stream;

/**
 * The Digest class provides a set of static final instances representing various cryptographic hash algorithms, along
 * with methods to compute the hexadecimal representation of byte arrays and streams using these algorithms.
 */
public class Digest {
    public static final Digest BLAKE_3 = new Digest(() -> MessageDigest.getInstance("BLAKE3-256", "BC"));
    public static final Digest SHA_256 = new Digest(() -> MessageDigest.getInstance("SHA-256"));
    public static final Digest SHA_384 = new Digest(() -> MessageDigest.getInstance("SHA-384"));
    public static final Digest SHA_512 = new Digest(() -> MessageDigest.getInstance("SHA-512"));
    private final MessageDigestSupplier supplier;

    /**
     * Constructs a new Digest instance with the specified {@link MessageDigestSupplier}.
     *
     * @param supplier The supplier for obtaining MessageDigest instances.
     */
    private Digest(final MessageDigestSupplier supplier) {
        this.supplier = supplier;
    }

    /**
     * Computes the hexadecimal representation of the given byte array using the configured hashing algorithm.
     *
     * @param data The input byte array to be hashed.
     * @return A string representing the hexadecimal digest of the input data
     */
    public String hexOf(final byte[] data) {
        return HexFormat.of().formatHex(instance().digest(data));
    }

    /**
     * Computes the hexadecimal representation of a stream of bytes using the configured hashing algorithm.
     *
     * @param stream The input stream of byte arrays to be hashed.
     * @return A string representing the hexadecimal digest of the input data stream
     */
    public String hexOf(final Stream<byte[]> stream) {
        MessageDigest digest = instance();
        stream.forEach(digest::update);
        return HexFormat.of().formatHex(digest.digest());
    }

    /**
     * Retrieves an instance of the configured {@link MessageDigest} for hashing operations.
     *
     * @return An instance of MessageDigest
     * @throws RuntimeException if the specified algorithm or provider is not available.
     */
    public MessageDigest instance() {
        try {
            return supplier.getInstance();
        } catch (NoSuchAlgorithmException | NoSuchProviderException e) {
            throw new RuntimeException(e);
        }
    }

    /** A functional interface for obtaining instances of {@link MessageDigest} with optional provider information. */
    private interface MessageDigestSupplier {
        /**
         * Obtains an instance of MessageDigest using the specified algorithm and optional provider.
         *
         * @return An instance of MessageDigest
         * @throws NoSuchAlgorithmException if the requested hash algorithm is not available.
         * @throws NoSuchProviderException if the specified security provider is not available.
         */
        MessageDigest getInstance() throws NoSuchAlgorithmException, NoSuchProviderException;
    }
}
