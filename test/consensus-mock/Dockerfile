# SPDX-License-Identifier: Apache-2.0

# Fast fake stand-in for the consensus-node container.

FROM debian:bookworm-slim AS selector

ARG TARGETARCH

COPY bin/consensus-mock-linux-* /tmp/

RUN cd /tmp && \
    sha256sum -c consensus-mock-linux-${TARGETARCH}.sha256 && \
    cp consensus-mock-linux-${TARGETARCH} /consensus-mock && \
    chmod +x /consensus-mock

FROM debian:bookworm-slim

# Real s6 (Debian's `s6` package) — internal/resources/consensus/exec.go execs s6-svc/s6-svstat here.
RUN apt-get update && \
    apt-get install -y --no-install-recommends s6 && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /package/admin/s6/command && \
    ln -s /usr/bin/s6-svc /package/admin/s6/command/s6-svc && \
    ln -s /usr/bin/s6-svstat /package/admin/s6/command/s6-svstat

RUN groupadd --gid 2000 hedera && \
    useradd --uid 2000 --gid 2000 --create-home --shell /bin/bash hedera

WORKDIR /

COPY --from=selector /consensus-mock .

# s6-svscan (PID 1) supervises this service so s6-svc/s6-svstat control a real process.
RUN mkdir -p /run/service/consensus && \
    printf '#!/bin/sh\nexec /consensus-mock\n' > /run/service/consensus/run && \
    chmod +x /run/service/consensus/run && \
    chown -R hedera:hedera /run/service

USER 2000:2000

ENTRYPOINT ["s6-svscan", "/run/service"]
