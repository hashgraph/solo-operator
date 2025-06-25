// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container.oci;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.Map;

/**
 * OCI Image Index Represents an OCI (Open Container Initiative) image index, which is a list of manifests for different
 * platforms and configurations.
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

    /**
     * Returns the media types supported by this class.
     *
     * @return An array of strings representing the supported media types
     */
    public static String[] mediaTypes() {
        return new String[] {
            "application/vnd.oci.image.index.v1+json", "application/vnd.docker.distribution.manifest.list.v2+json"
        };
    }

    /** Represents a manifest for an image indexed by the OCI Image Index. */
    public static class OCIImageIndexManifest extends OCIContentDescriptor {
        public OCIImageIndexPlatform platform;
    }

    /** Represents the platform configuration for an image or artifact. */
    public static class OCIImageIndexPlatform {
        public String architecture;
        public String os;

        @JsonProperty("os.version")
        public String os_version;

        @JsonProperty("os.features")
        public String[] os_features;

        public String variant;
        public String[] features;

        /** Default constructor for OCIImageIndexPlatform. */
        public OCIImageIndexPlatform() {}

        /**
         * Constructor to initialize a new OCIImageIndexPlatform with OS and architecture.
         *
         * @param os The operating system.
         * @param architecture The hardware architecture.
         */
        public OCIImageIndexPlatform(String os, String architecture) {
            this.os = os;
            this.architecture = architecture;
        }
    }
}
