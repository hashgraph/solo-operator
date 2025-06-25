// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container.oci;

import java.util.Map;

/**
 * OCI Content Descriptors Represents the content descriptor for an OCI (Open Container Initiative) artifact, such as a
 * container image.
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

    /** Default constructor for OCIContentDescriptor. */
    public OCIContentDescriptor() {}

    /**
     * Private constructor to create an instance of OCIContentDescriptor using the Builder.
     *
     * @param builder The Builder instance containing the properties to set.
     */
    private OCIContentDescriptor(Builder builder) {
        this.mediaType = builder.mediaType;
        this.digest = builder.digest;
        this.size = builder.size;
        this.urls = builder.urls;
        this.annotations = builder.annotations;
        this.data = builder.data;
        this.artifactType = builder.artifactType;
    }

    /**
     * Get a new Builder instance to construct an OCIContentDescriptor.
     *
     * @return A Builder instance for constructing the descriptor
     */
    public static Builder builder() {
        return new Builder();
    }

    /** Inner class for building instances of OCIContentDescriptor. */
    public static class Builder {
        private String mediaType;
        private String digest;
        private Long size;
        private String[] urls;
        private Map<String, String> annotations;
        private String data;
        private String artifactType;

        /**
         * Set the media type of the content descriptor.
         *
         * @param mediaType The media type to set.
         * @return This Builder instance for method chaining
         */
        public Builder withMediaType(String mediaType) {
            this.mediaType = mediaType;
            return this;
        }

        /**
         * Set the digest of the content descriptor.
         *
         * @param digest The digest to set.
         * @return This Builder instance for method chaining
         */
        public Builder withDigest(String digest) {
            this.digest = digest;
            return this;
        }

        /**
         * Set the size of the content descriptor.
         *
         * @param size The size to set.
         * @return This Builder instance for method chaining
         */
        public Builder withSize(Long size) {
            this.size = size;
            return this;
        }

        /**
         * Set the URLs of the content descriptor.
         *
         * @param urls The array of URLs to set.
         * @return This Builder instance for method chaining
         */
        public Builder withUrls(String[] urls) {
            this.urls = urls;
            return this;
        }

        /**
         * Set the annotations of the content descriptor.
         *
         * @param annotations The map of annotations to set.
         * @return This Builder instance for method chaining
         */
        public Builder withAnnotations(Map<String, String> annotations) {
            this.annotations = annotations;
            return this;
        }

        /**
         * Set the data content of the descriptor.
         *
         * @param data The data to set.
         * @return This Builder instance for method chaining
         */
        public Builder withData(String data) {
            this.data = data;
            return this;
        }

        /**
         * Set the artifact type of the content descriptor.
         *
         * @param artifactType The artifact type to set.
         * @return This Builder instance for method chaining
         */
        public Builder withArtifactType(String artifactType) {
            this.artifactType = artifactType;
            return this;
        }

        /**
         * Build and return the OCIContentDescriptor instance.
         *
         * @return The constructed OCIContentDescriptor instance
         */
        public OCIContentDescriptor build() {
            return new OCIContentDescriptor(this);
        }
    }
}
