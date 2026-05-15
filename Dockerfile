# SPDX-License-Identifier: Apache-2.0

# Selector stage: pick the pre-compiled binary for the target architecture
FROM debian:bookworm-slim AS selector

COPY bin/manager-linux-* /tmp/

RUN ARCH=$(uname -m) && \
    if [ "${ARCH}" = "x86_64" ]; then ARCH="amd64"; \
    elif [ "${ARCH}" = "aarch64" ]; then ARCH="arm64"; fi && \
    cd /tmp && \
    sha256sum -c manager-linux-${ARCH}.sha256 && \
    cp manager-linux-${ARCH} /manager && \
    chmod +x /manager

# Final stage
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /

COPY --from=selector /manager .

USER 65532:65532

ENTRYPOINT ["/manager"]
