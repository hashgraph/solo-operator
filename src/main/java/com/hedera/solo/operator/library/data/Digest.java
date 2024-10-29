// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.data;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;
import java.util.Map;

public class Digest {

    public static final String SHA_384 = "SHA-384";

    private Digest() {}

    public static String of(final Map<String, String> map) {
        try {
            MessageDigest digest = MessageDigest.getInstance(SHA_384);
            map.entrySet().stream().sorted(Map.Entry.comparingByKey()).forEach(entry -> {
                digest.update(entry.getKey().getBytes());
                digest.update(entry.getValue().getBytes());
            });
            return HexFormat.of().formatHex(digest.digest());
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException(e);
        }
    }
}
