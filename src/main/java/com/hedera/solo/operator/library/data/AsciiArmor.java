// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.data;

import static java.nio.charset.StandardCharsets.UTF_8;
import static java.util.Base64.getDecoder;
import static java.util.Base64.getEncoder;

import java.nio.charset.Charset;

/** The {@code AsciiArmor} class provides utility methods for encoding and decoding strings using Base64 encoding. */
public class AsciiArmor {
    /** Private constructor to prevent instantiation of this utility class. */
    private AsciiArmor() {}

    /**
     * Decodes a Base64 encoded string to its original form with the default charset UTF-8.
     *
     * @param string the Base64 encoded string to be decoded
     * @return the decoded string in its original form
     */
    public static String base64Decode(final String string) {
        return base64Decode(UTF_8, string);
    }

    /**
     * Encodes a string to Base64 format with the default charset UTF-8.
     *
     * @param string the string to be encoded
     * @return the Base64 encoded string
     */
    public static String base64Encode(final String string) {
        return base64Encode(UTF_8, string);
    }

    /**
     * Decodes a Base64 encoded string to its original form using the specified charset.
     *
     * @param charset the character set to use for decoding
     * @param string the Base64 encoded string to be decoded
     * @return the decoded string in its original form
     */
    public static String base64Decode(final Charset charset, final String string) {
        return new String(getDecoder().decode(string), charset);
    }

    /**
     * Encodes a string to Base64 format using the specified charset.
     *
     * @param charset the character set to use for encoding
     * @param string the string to be encoded
     * @return the Base64 encoded string
     */
    public static String base64Encode(final Charset charset, final String string) {
        return getEncoder().encodeToString(string.getBytes(charset));
    }
}
