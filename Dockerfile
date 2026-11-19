# SPDX-License-Identifier: Apache-2.0

# Selector stage: pick the pre-compiled binary for the target architecture.
# TARGETARCH is set by docker buildx to the target platform (amd64/arm64),
# which matches our binary suffix — unlike uname -m which reflects the builder.
FROM debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS selector

ARG TARGETARCH

COPY bin/manager-linux-* /tmp/

RUN cd /tmp && \
    sha256sum -c manager-linux-${TARGETARCH}.sha256 && \
    cp manager-linux-${TARGETARCH} /manager && \
    chmod +x /manager

# Final stage
FROM gcr.io/distroless/static-debian13:nonroot@sha256:aafe993665bf6200db38a02c11a9351e782a84b6a45e7f33b6695638c88e3931

WORKDIR /

COPY --from=selector /manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
