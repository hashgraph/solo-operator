// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container.oci;

import java.util.Map;

/**
 * OCI Image Manifest Represents the manifest for an OCI (Open Container Initiative) image. This class follows the
 * specification defined in the OCI Image Manifest Specification.
 *
 * @see <a href="https://github.com/opencontainers/image-spec/blob/main/manifest.md">OCI Image Manifest
 *     Specification</a>
 */
public final class OCIImageManifest extends OCIManifest {
    public Long schemaVersion;
    public String mediaType;
    public String artifactType;
    public OCIContentDescriptor config;
    public OCIContentDescriptor[] layers;
    public OCIContentDescriptor subject;
    public Map<String, String> annotations;

    /**
     * Returns the media types supported by this manifest.
     *
     * @return Array of supported media types
     */
    public static String[] mediaTypes() {
        return new String[] {
            "application/vnd.oci.image.manifest.v1+json", "application/vnd.docker.distribution.manifest.v2+json"
        };
    }
}
