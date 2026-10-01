#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Builds the upgrade package (zip + SHA-384) from a local consensus-node build, with a
# consensus-node-components.yaml manifest whose layer hashes come from the local docker image.
# Required env: CN_DIR CN_IMG CN_NAME CN_V2 REGISTRY_CLUSTER_ADDR OVERLAY_DIR OUT_DIR
set -euo pipefail

: "${CN_DIR:?}" "${CN_IMG:?}" "${CN_NAME:?}" "${CN_V2:?}" "${REGISTRY_CLUSTER_ADDR:?}" "${OVERLAY_DIR:?}" "${OUT_DIR:?}"

err() { echo "ERROR: $*" >&2; exit 1; }

[ -d "$CN_DIR/hedera-node/data/apps" ] && [ -d "$CN_DIR/hedera-node/data/lib" ] ||
  err "no consensus-node build output under $CN_DIR/hedera-node/data; run 'task build' first"
[ -d "$OVERLAY_DIR/data/config" ] || err "overlay $OVERLAY_DIR not found"
command -v zip >/dev/null || err "zip not found"

docker image inspect "$CN_IMG" >/dev/null 2>&1 || err "image $CN_IMG not found locally; run 'task build' first"
platform="$(docker image inspect "$CN_IMG" --format '{{.Os}}/{{.Architecture}}')"
layers="$(docker image inspect "$CN_IMG" --format '{{range .RootFS.Layers}}{{.}}{{"\n"}}{{end}}')"
[ -n "$layers" ] || err "image $CN_IMG has no RootFS layers"

PKG="$OUT_DIR/package"
ZIP="$OUT_DIR/softwareUpgrade.zip"
rm -rf "$PKG" "$ZIP"
mkdir -p "$PKG/data/config" "$PKG/manifests" "$PKG/block-nodes/config"

cp -R "$CN_DIR/hedera-node/data/apps" "$CN_DIR/hedera-node/data/lib" "$PKG/data/"
cp "$CN_DIR/version.txt" "$PKG/VERSION"

# Only the HIP-1494 config set; other files make the UC abort at execute time.
for f in application.properties application-override.properties api-permission.properties bootstrap.properties \
  node.properties feeSchedules.json simpleFeesSchedules.json throttles.json genesis-network.json; do
  [ -f "$OVERLAY_DIR/data/config/$f" ] && cp "$OVERLAY_DIR/data/config/$f" "$PKG/data/config/"
done
[ -f "$PKG/data/config/genesis-network.json" ] || cp "$OVERLAY_DIR/genesis-network.json" "$PKG/data/config/"
if [ -f "$PKG/data/config/bootstrap.properties" ]; then
  grep -v '^contracts.maxNumWithHapiSigsAccess=' "$PKG/data/config/bootstrap.properties" >"$PKG/bootstrap.tmp" || true
  mv "$PKG/bootstrap.tmp" "$PKG/data/config/bootstrap.properties"
fi
for nid in 0 1 2; do
  printf '{"nodes":[]}\n' >"$PKG/block-nodes/config/block-nodes-${nid}.json"
done

{
  echo "schemaVersion: 1"
  echo "images:"
  echo "  consensusNode:"
  echo "    version: \"$CN_V2\""
  echo "    deterministic:"
  echo "      supported: true"
  echo "      layerHashes:"
  echo "        $platform:"
  while IFS= read -r d; do echo "          - \"$d\""; done <<<"$layers"
  echo "    registries:"
  echo "      - image: \"$REGISTRY_CLUSTER_ADDR/$CN_NAME:$CN_V2\""
} >"$PKG/manifests/consensus-node-components.yaml"

(cd "$PKG" && zip -qr -X "../softwareUpgrade.zip" .)
shasum -a 384 "$ZIP" | awk '{print $1}' >"$ZIP.sha384"

echo "zip:  $ZIP ($(du -h "$ZIP" | cut -f1))"
echo "H:    $(cat "$ZIP.sha384")"
echo "image layers ($platform): $(printf '%s\n' "$layers" | grep -c .)"
