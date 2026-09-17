# SPDX-License-Identifier: Apache-2.0

# Dockerfile for consensus-mock, a fast fake stand-in for the consensus-node
# container used to time the UC-driven upgrade flow in tests. Mirrors
# Dockerfile.uc: a selector stage picks the pre-compiled arch binary, then the
# final stage runs it as the same hedera 2000:2000 user the real consensus
# node and the UC sidecar use, since consensus-mock shares the upgrade/ and
# output/ volumes with both.

FROM debian:bookworm-slim AS selector

ARG TARGETARCH

COPY bin/consensus-mock-linux-* /tmp/

RUN cd /tmp && \
    sha256sum -c consensus-mock-linux-${TARGETARCH}.sha256 && \
    cp consensus-mock-linux-${TARGETARCH} /consensus-mock && \
    chmod +x /consensus-mock

FROM debian:bookworm-slim

RUN groupadd --gid 2000 hedera && \
    useradd --uid 2000 --gid 2000 --create-home --shell /bin/bash hedera

WORKDIR /

COPY --from=selector /consensus-mock .

# internal/resources/consensus/exec.go's ActuationSubReconciler execs these on
# EVERY ConsensusCapsule reconcile (Auto or Manual) to query/actuate the real
# image's s6-supervised process — without them the reconciler errors on every
# pass. consensus-mock has no supervision tree (its one process is always
# "up" for as long as the container runs), so these are trivial stubs: -o up
# always reports "true", and start/stop are no-ops.
RUN mkdir -p /package/admin/s6/command && \
    printf '#!/bin/sh\necho true\n' > /package/admin/s6/command/s6-svstat && \
    printf '#!/bin/sh\nexit 0\n' > /package/admin/s6/command/s6-svc && \
    chmod +x /package/admin/s6/command/s6-svstat /package/admin/s6/command/s6-svc

USER 2000:2000

ENTRYPOINT ["/consensus-mock"]
