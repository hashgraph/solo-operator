# SPDX-License-Identifier: Apache-2.0

# Selector stage: pick the pre-compiled binary for the target architecture.
# TARGETARCH is set by docker buildx to the target platform (amd64/arm64),
# which matches our binary suffix — unlike uname -m which reflects the builder.
FROM debian:bookworm-slim AS selector

ARG TARGETARCH

COPY bin/manager-linux-* /tmp/

RUN cd /tmp && \
    sha256sum -c manager-linux-${TARGETARCH}.sha256 && \
    cp manager-linux-${TARGETARCH} /manager && \
    chmod +x /manager

# Final stage
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /

COPY --from=selector /manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
