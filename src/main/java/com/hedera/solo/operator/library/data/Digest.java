// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.data;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.NoSuchProviderException;
import java.util.HexFormat;
import java.util.stream.Stream;

public class Digest {
    public static final Digest BLAKE_3 = new Digest(() -> MessageDigest.getInstance("BLAKE3-256", "BC"));
    public static final Digest SHA_256 = new Digest(() -> MessageDigest.getInstance("SHA-256"));
    public static final Digest SHA_384 = new Digest(() -> MessageDigest.getInstance("SHA-384"));
    public static final Digest SHA_512 = new Digest(() -> MessageDigest.getInstance("SHA-512"));
    private final MessageDigestSupplier supplier;

    private Digest(final MessageDigestSupplier supplier) {
        this.supplier = supplier;
    }

    public String hexOf(final byte[] data) {
        return HexFormat.of().formatHex(instance().digest(data));
    }

    public String hexOf(final Stream<byte[]> stream) {
        MessageDigest digest = instance();
        stream.forEach(digest::update);
        return HexFormat.of().formatHex(digest.digest());
    }

    public MessageDigest instance() {
        try {
            return supplier.getInstance();
        } catch (NoSuchAlgorithmException | NoSuchProviderException e) {
            throw new RuntimeException(e);
        }
    }

    private interface MessageDigestSupplier {
        MessageDigest getInstance() throws NoSuchAlgorithmException, NoSuchProviderException;
    }
}
