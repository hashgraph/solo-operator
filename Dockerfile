# SPDX-License-Identifier: Apache-2.0

# Selector stage: pick the pre-compiled binary for the target architecture.
# TARGETARCH is set by docker buildx to the target platform (amd64/arm64),
# which matches our binary suffix — unlike uname -m which reflects the builder.
FROM debian:bookworm-slim@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8 AS selector

ARG TARGETARCH

COPY bin/manager-linux-* /tmp/

RUN cd /tmp && \
    sha256sum -c manager-linux-${TARGETARCH}.sha256 && \
    cp manager-linux-${TARGETARCH} /manager && \
    chmod +x /manager

# Final stage
FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99

WORKDIR /

COPY --from=selector /manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
