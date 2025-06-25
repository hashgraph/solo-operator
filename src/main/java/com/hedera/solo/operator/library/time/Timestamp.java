// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.time;

import java.time.ZonedDateTime;
import java.time.format.DateTimeFormatter;

/** The Timestamp class provides a utility method to generate a timestamp string in ISO format. */
public class Timestamp {
    /** Private constructor to prevent instantiation of this utility class. */
    private Timestamp() {}

    /**
     * Generates and returns the current time as an ISO-formatted timestamp string, with nanoseconds omitted.
     *
     * @return A String representing the current time in ISO-8601 format without nanoseconds
     */
    public static String timestamp() {
        return DateTimeFormatter.ISO_INSTANT.format(ZonedDateTime.now().withNano(0));
    }
}
