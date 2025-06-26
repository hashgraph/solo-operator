// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.unit;

import static com.hedera.solo.operator.library.data.AsciiArmor.base64Encode;
import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.library.data.AsciiArmor;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

/**
 * This class contains unit tests for the ImageCoordinates class, which handles image coordinates in a Docker registry.
 */
public class ImageCoordinatesTest {
    @Test
    void testInvalidImageCoordinates() {
        assertThrows(IllegalArgumentException.class, () -> ImageCoordinates.https("."));
        assertThrows(IllegalArgumentException.class, () -> ImageCoordinates.https("host/path", "{'auths':{}}"));
    }

    @Test
    void testImageCoordinatesEquality() {
        ImageCoordinates imageCoordinates = ImageCoordinates.https("host/path");
        assertThat(imageCoordinates.equals(imageCoordinates)).isTrue();
        assertThat(imageCoordinates.equals(ImageCoordinates.https("host/path"))).isTrue();
        assertThat(imageCoordinates.equals(ImageCoordinates.https("host1/path")))
                .isFalse();
        assertThat(imageCoordinates.equals(ImageCoordinates.https("host/path1")))
                .isFalse();
        assertThat(imageCoordinates.equals(
                        ImageCoordinates.https("host/path", "{\"auths\":{\"host\":{\"auth\":\"YTpi\"}}}")))
                .isFalse();
        assertThat(imageCoordinates.equals(null)).isFalse();
        assertThat(imageCoordinates.equals("")).isFalse();
    }

    @Test
    void testImageCoordinatesWithBlankReference() {
        ImageCoordinates imageCoordinates = ImageCoordinates.https("host/path:");
        assertThat(imageCoordinates.reference()).isEqualTo("latest");
    }

    @Test
    void testAllMatchingAuths() throws JsonProcessingException {
        Config config = new Config();
        config.auths.put("host", new Entry("user", "host"));
        config.auths.put("host/path", new Entry("user", "host/path"));
        config.auths.put("host/path/name", new Entry("user", "host/path/name"));
        String auth = new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(config);
        checkPasswords(auth, "host/", "host");
        checkPasswords(auth, "host/path", "host", "host/path");
        checkPasswords(auth, "host/path/name", "host", "host/path", "host/path/name");
        checkPasswords(auth, "host/name", "host");
        checkPasswords(auth, "host/path2/name", "host", "host/path");
        checkPasswords(auth, "host/path/name2", "host", "host/path", "host/path/name");
        checkPasswords(auth, "host/path/name/2", "host", "host/path", "host/path/name");
    }

    /** Represents the configuration for image coordinates, containing a map of authorization entries. */
    static class Config {
        public Map<String, Entry> auths = new LinkedHashMap<>();
    }

    /**
     * Represents an entry in the authorization configuration, containing a base64 encoded string of the username and
     * password.
     */
    static class Entry {
        public String auth;

        /**
         * Constructs an Entry with the given username and password, encoding them in base64 format.
         *
         * @param username the username to be encoded
         * @param password the password to be encoded
         */
        Entry(final String username, final String password) {
            this.auth = base64Encode(username + ":" + password);
        }
    }

    /**
     * Converts a collection of authorization strings to their decoded passwords.
     *
     * @param auths the collection of authorization strings in the format "username:password" (base64 encoded)
     * @return an array of passwords extracted from the authorization strings
     */
    String[] passwords(final Collection<String> auths) {
        return auths.stream()
                .map(AsciiArmor::base64Decode)
                .map(string -> string.substring(string.indexOf(':') + 1))
                .toArray(String[]::new);
    }

    /**
     * Verifies that the passwords are correctly extracted from the given authorization configuration and image
     * references.
     *
     * @param auth the authorization configuration in JSON format
     * @param image the image reference
     * @param passwords the expected passwords to be extracted
     */
    void checkPasswords(final String auth, final String image, final String... passwords) {
        for (String tag : List.of("", ":latest", ":1.0")) {
            assertThat(passwords(ImageCoordinates.https(image + tag, auth).auths))
                    .isEqualTo(passwords);
        }
    }
}
