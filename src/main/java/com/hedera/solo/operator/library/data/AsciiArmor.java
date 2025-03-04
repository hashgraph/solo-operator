// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.data;

import static java.nio.charset.StandardCharsets.UTF_8;
import static java.util.Base64.getDecoder;
import static java.util.Base64.getEncoder;

import java.nio.charset.Charset;

public class AsciiArmor {
    private AsciiArmor() {}

    public static String base64Decode(final String string) {
        return base64Decode(UTF_8, string);
    }

    public static String base64Encode(final String string) {
        return base64Encode(UTF_8, string);
    }

    public static String base64Decode(final Charset charset, final String string) {
        return new String(getDecoder().decode(string), charset);
    }

    public static String base64Encode(final Charset charset, final String string) {
        return getEncoder().encodeToString(string.getBytes(charset));
    }
}
