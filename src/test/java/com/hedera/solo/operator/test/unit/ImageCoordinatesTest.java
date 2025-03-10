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

    static class Config {
        public Map<String, Entry> auths = new LinkedHashMap<>();
    }

    static class Entry {
        public String auth;

        Entry(final String username, final String password) {
            this.auth = base64Encode(username + ":" + password);
        }
    }

    String[] passwords(final Collection<String> auths) {
        return auths.stream()
                .map(AsciiArmor::base64Decode)
                .map(string -> string.substring(string.indexOf(':') + 1))
                .toArray(String[]::new);
    }

    void checkPasswords(final String auth, final String image, final String... passwords) {
        for (String tag : List.of("", ":latest", ":1.0")) {
            assertThat(passwords(ImageCoordinates.https(image + tag, auth).auths))
                    .isEqualTo(passwords);
        }
    }
}
