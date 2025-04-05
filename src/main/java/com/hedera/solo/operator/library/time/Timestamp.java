// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.time;

import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;

public class Timestamp {
    private Timestamp() {}

    public static String timestamp() {
        return DateTimeFormatter.ISO_INSTANT.format(ZonedDateTime.now().withNano(0));
    }
}
