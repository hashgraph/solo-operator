# SPDX-License-Identifier: Apache-2.0

# Dockerfile for the Upgrade Controller (UC) sidecar. Published as a separate
# image with its own release cadence (HIP XXXX2 §902). Mirrors the operator
# Dockerfile: a selector stage picks the pre-compiled arch binary, then a
# distroless final stage runs it.

# Selector stage: pick the pre-compiled binary for the target architecture.
# TARGETARCH is set by docker buildx to the target platform (amd64/arm64),
# which matches our binary suffix — unlike uname -m which reflects the builder.
FROM debian:bookworm-slim AS selector

ARG TARGETARCH

COPY bin/uc-linux-* /tmp/

RUN cd /tmp && \
    sha256sum -c uc-linux-${TARGETARCH}.sha256 && \
    cp uc-linux-${TARGETARCH} /uc && \
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
