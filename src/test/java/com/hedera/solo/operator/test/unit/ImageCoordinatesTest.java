// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.unit;

import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.hedera.solo.operator.library.container.ImageCoordinates;
import org.junit.jupiter.api.Test;

public class ImageCoordinatesTest {
    @Test
    void testInvalidImageCoordinates() {
        assertThrows(IllegalArgumentException.class, () -> ImageCoordinates.http("."));
        assertThrows(IllegalArgumentException.class, () -> ImageCoordinates.http("host/path", "{'auths':{}}"));
        assertThrows(
                IllegalArgumentException.class, () -> ImageCoordinates.http("host/path", "{\"auths\":{\"host\":{}}}"));
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
}
