// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container.oci;

import java.util.Map;

/**
 * OCI Content Descriptors
 *
 * @see <a href="https://github.com/opencontainers/image-spec/blob/main/descriptor.md">OCI Content Descriptors</a>
 */
public class OCIContentDescriptor {
    public String mediaType;
    public String digest;
    public Long size;
    public String[] urls;
    public Map<String, String> annotations;
    public String data;
    public String artifactType;
}
