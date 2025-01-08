// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container.oci;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Map;

/**
 * OCI Image Index
 *
 * @see <a href="https://github.com/opencontainers/image-spec/blob/main/image-index.md">OCI Image Index
 *     Specification</a>
 */
public final class OCIImageIndex extends OCIManifest {
    public Long schemaVersion;
    public String mediaType;
    public String artifactType;
    public OCIImageIndexManifest[] manifests;
    public OCIContentDescriptor subject;
    public Map<String, String> annotations;

    public static String[] mediaTypes() {
        return new String[] {
            "application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json"
        };
    }

    public static class OCIImageIndexManifest extends OCIContentDescriptor {
        public OCIImageIndexPlatform platform;
    }

    public static class OCIImageIndexPlatform {
        public String architecture;
        public String os;

        public OCIImageIndexPlatform() {}

        public OCIImageIndexPlatform(final String os, final String architecture) {
            this.os = os;
            this.architecture = architecture;
        }

        @JsonProperty("os.version")
        public String os_version;

        @JsonProperty("os.features")
        public String[] os_features;

        public String variant;
        public String[] features;
    }
}
