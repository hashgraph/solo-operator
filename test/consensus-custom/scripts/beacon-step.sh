#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Applies a beacon Job manifest (envsubst of ZIP_HASH/START_TIME), waits for it, then optionally
# waits for the new NetworkUpgrade* CR it triggers to reach Succeeded.
# Usage: beacon-step.sh <job> <manifest> <timeout-secs> [cr-resource]
# Env: NS (default solo-orbit), ZIP_HASH, START_TIME, LOG_COPY (file to tee the job log into)
set -euo pipefail

JOB="${1:?job}"
FILE="${2:?manifest}"
TIMEOUT="${3:?timeout-secs}"
CR="${4:-}"
NS="${NS:-solo-orbit}"

list_crs() { kubectl get "$CR" -n "$NS" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true; }

BASELINE=""
[ -z "$CR" ] || BASELINE="$(list_crs)"

kubectl delete job "$JOB" -n "$NS" --ignore-not-found >/dev/null
envsubst '${ZIP_HASH} ${START_TIME}' <"$FILE" | kubectl apply -f -

if ! kubectl wait --for=condition=complete --timeout="${TIMEOUT}s" "job/$JOB" -n "$NS"; then
  echo "ERROR: job/$JOB did not complete within ${TIMEOUT}s; log follows" >&2
  kubectl logs -n "$NS" "job/$JOB" >&2 || true
  exit 1
fi
kubectl logs -n "$NS" "job/$JOB" | tee "${LOG_COPY:-/dev/null}"

[ -n "$CR" ] || exit 0

echo ">> waiting for a new $CR CR..."
NAME=""
for _ in $(seq 1 "$TIMEOUT"); do
  for n in $(list_crs); do
    case " $BASELINE " in *" $n "*) ;; *) NAME="$n" ;; esac
  done
  [ -z "$NAME" ] || break
  sleep 1
done
[ -n "$NAME" ] || { echo "ERROR: no new $CR CR appeared within ${TIMEOUT}s" >&2; exit 1; }

echo ">>   $CR/$NAME - waiting for Succeeded..."
kubectl wait --for=jsonpath='{.status.phase}'=Succeeded "$CR/$NAME" -n "$NS" --timeout="${TIMEOUT}s"
