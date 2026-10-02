#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Output paths match what consensus-mock's wait:execute reads.
set -euo pipefail

NS="${NS:-solo-orbit}"
PREFIX=/tmp/consensus-mock-freeze-window

rm -f "$PREFIX"-*.log
nohup kubectl logs -f node0-0 -c consensus-node -n "$NS" >"$PREFIX-consensus-node.log" 2>&1 &
nohup kubectl logs -f node0-0 -c uc -n "$NS" >"$PREFIX-uc.log" 2>&1 &
nohup kubectl logs -f -l app.kubernetes.io/name=solo-operator -n solo-operator >"$PREFIX-operator.log" 2>&1 &
