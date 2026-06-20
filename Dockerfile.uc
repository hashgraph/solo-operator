# SPDX-License-Identifier: Apache-2.0

# Dockerfile for the Upgrade Controller (UC) sidecar. Published as a separate
# image with its own release cadence (HIP XXXX2 §902). Mirrors the operator
# Dockerfile: a selector stage picks the pre-compiled arch binary, then a
# distroless final stage runs it.

# Selector stage: pick the pre-compiled binary for the target architecture
FROM debian:bookworm-slim AS selector

COPY bin/uc-linux-* /tmp/

RUN ARCH=$(uname -m) && \
    if [ "${ARCH}" = "x86_64" ]; then ARCH="amd64"; \
    elif [ "${ARCH}" = "aarch64" ]; then ARCH="arm64"; fi && \
    cd /tmp && \
    sha256sum -c uc-linux-${ARCH}.sha256 && \
    cp uc-linux-${ARCH} /uc && \
    chmod +x /uc

# Final stage
FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /

COPY --from=selector /uc .

# The effective runtime user is set by the pod securityContext so the sidecar
# can write the CN-owned upgrade directory (hedera group). 65532 is the image
# default for non-Kubernetes runs.
USER 65532:65532

ENTRYPOINT ["/uc"]
