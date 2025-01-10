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

    public OCIContentDescriptor() {}

    private OCIContentDescriptor(Builder builder) {
        this.mediaType = builder.mediaType;
        this.digest = builder.digest;
        this.size = builder.size;
        this.urls = builder.urls;
        this.annotations = builder.annotations;
        this.data = builder.data;
        this.artifactType = builder.artifactType;
    }

    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private String mediaType;
        private String digest;
        private Long size;
        private String[] urls;
        private Map<String, String> annotations;
        private String data;
        private String artifactType;

        public Builder withMediaType(String mediaType) {
            this.mediaType = mediaType;
            return this;
        }

        public Builder withDigest(String digest) {
            this.digest = digest;
            return this;
        }

        public Builder withSize(Long size) {
            this.size = size;
            return this;
        }

        public Builder withUrls(String[] urls) {
            this.urls = urls;
            return this;
        }

        public Builder withAnnotations(Map<String, String> annotations) {
            this.annotations = annotations;
            return this;
        }

        public Builder withData(String data) {
            this.data = data;
            return this;
        }

        public Builder withArtifactType(String artifactType) {
            this.artifactType = artifactType;
            return this;
        }

        public OCIContentDescriptor build() {
            return new OCIContentDescriptor(this);
        }
    }
}
