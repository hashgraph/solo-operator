// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container;

import com.hedera.solo.operator.library.container.oci.OCIManifest;
import com.hedera.solo.operator.library.data.Digest;
import java.net.http.HttpResponse;
import java.util.HashMap;
import java.util.Map;
import java.util.TreeMap;

/** Manifest response from registry including unmarshalled manifest for recognized media types. */
public class ManifestResponse {
    public static final Map<String, Digest> digestAlgorithms = digestAlgorithms();
    public final HttpResponse<byte[]> response;
    public final OCIManifest manifest;
    public final String digest;
    protected final Map<String, String> digests = new HashMap<>();

    public ManifestResponse(final ImageCoordinates coordinates, final HttpResponse<byte[]> response) {
        this.response = response;
        requireDigest(
                digest = coordinates.digest != null
                        ? coordinates.digest
                        : response.headers().firstValue("Docker-Content-Digest").orElseGet(() -> digest("sha256")));
        manifest =
                OCIManifest.from(response.headers().firstValue("Content-Type").orElse(null), response.body());
    }

    public String digest(final String algorithm) {
        return digests.computeIfAbsent(
                algorithm, k -> k + ":" + digestAlgorithms.get(algorithm).hexOf(response.body()));
    }

    protected void requireDigest(final String expect) {
        String actual = digest(expect.substring(0, 6));
        if (!actual.equalsIgnoreCase(expect)) {
            throw new RuntimeException(
                    "Data integrity violation encountered. Expected: " + expect + ", Actual: " + actual);
        }
    }

    private static Map<String, Digest> digestAlgorithms() {
        Map<String, Digest> digestAlgorithms = new TreeMap<>(String.CASE_INSENSITIVE_ORDER);
        digestAlgorithms.put("blake3", Digest.BLAKE_3);
        digestAlgorithms.put("sha256", Digest.SHA_256);
        digestAlgorithms.put("sha384", Digest.SHA_384);
        digestAlgorithms.put("sha512", Digest.SHA_512);
        return digestAlgorithms;
    }
}
