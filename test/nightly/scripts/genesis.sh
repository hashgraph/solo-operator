#! /usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

# Get the directory of the current script
bin=$(dirname "$(readlink -f -- "${BASH_SOURCE}")")

. ${bin}/error.sh

genesis() {
  name=$(hostname -s | sed 's,^\S*-,,')
  base_dir=test
  node_dir="${base_dir}/${name}"
  sudo "${bin}/create_hapi_folders.sh"
  kubectl apply -f "${node_dir}/orbit.yaml"
  kubectl wait -n solo-orbit --for=create --timeout=1m ns/solo-orbit
  kubectl wait -n solo-orbit --for=jsonpath=status.phase=Active --timeout=1m ns/solo-orbit
  kubectl apply -f "${node_dir}/secrets.yaml"
  kubectl apply -f "${node_dir}/consensus-capsule.yaml"
  kubectl wait -n solo-orbit --for=create consensuscapsules.operator.solo.hedera.com/${name}  --timeout=5m
  kubectl apply -f "${base_dir}/network-operation/network-genesis.yaml"
  kubectl wait -n solo-orbit --for=jsonpath=status.phase=Succeeded --timeout=10m networkoperations.operator.solo.hedera.com/network-genesis
}

genesis
