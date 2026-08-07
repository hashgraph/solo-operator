# SPDX-License-Identifier: Apache-2.0

# Dockerfile for the Upgrade Controller (UC) sidecar. Published as a separate
# image with its own release cadence (HIP XXXX2 §902). Mirrors the operator
# Dockerfile: a selector stage picks the pre-compiled arch binary, then the
# final stage runs it.

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

# Final stage.
#
# The UC shares the upgrade PVC (data/upgrade) with the consensus node, which
# creates and extracts the upgrade package under current/ as the hedera user.
# The UC must therefore run as the SAME uid:gid as the consensus node (hedera,
# 2000:2000) so it can create/rotate current/ and previous/ and the node can
# write into them — a distroless nonroot (65532) UC left those dirs owned by a
# foreign uid, so the node's extraction failed with mkdir "Permission denied".
#
# We use debian:bookworm-slim (not distroless) so the image ships a shell for
# `kubectl exec` debugging of the shared upgrade tree.
FROM debian:bookworm-slim

# Match the consensus-node image: hedera group and user, both 2000.
RUN groupadd --gid 2000 hedera && \
    useradd --uid 2000 --gid 2000 --create-home --shell /bin/bash hedera

WORKDIR /

COPY --from=selector /uc .

USER 2000:2000

ENTRYPOINT ["/uc"]
