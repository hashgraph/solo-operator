#! /usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# This script captures logs and resources from a Kubernetes cluster.

# Function to capture non-namespaced resources
non_namespaced() {
  ARGS="${*:-.}"
  ARGS="${ARGS// /\\|}"
  for name in $(kubectl api-resources --verbs=list --namespaced=false -o name | grep "${ARGS}" | xargs -n 1 kubectl get --ignore-not-found -o name); do
    file=non-namespaced/${name}
    mkdir -p $(dirname ${file})
    kubectl get --ignore-not-found -o yaml ${name} > ${file}.yaml
  done
}

# Function to capture namespaced resources
namespaced() {
  ns="${1}"
  for name in $(kubectl api-resources --verbs=list --namespaced -o name | xargs -n 1 kubectl get --ignore-not-found -n ${ns} -o name); do
    file=namespaced/${ns}/${name}
    mkdir -p $(dirname ${file})
    kubectl get --ignore-not-found -n ${ns} -o yaml ${name} > ${file}.yaml
  done
}

# Function to capture logs from all pods in a namespace
logs() {
  ns="${1}"
  for pod in $(kubectl get pods -n ${ns} -o name); do
    for container in $(kubectl get ${pod} -n ${ns} -o jsonpath="{.spec['containers','initContainers'][*].name}"); do
      dir=logs/${ns}/${pod}/${container}
      mkdir -p ${dir}
      kubectl logs ${pod} -c ${container} --timestamps -n ${ns} -p > ${dir}/dead.log || rm ${dir}/dead.log
      kubectl logs ${pod} -c ${container} --timestamps -n ${ns} > ${dir}/live.log
    done
  done
}

# Function to copy files from a specific container in all pods matching the labels
files() {
  ns="${1}"
  labels="${2}"
  name="${3}"
  shift 3
  for pod in $(kubectl get pods -n ${ns} -l "${labels}" -o name); do
    for container in $(kubectl get ${pod} -n ${ns} -o jsonpath="{.spec['containers','initContainers'][*].name}"); do
      if [ "${container}" == "${name}" ]; then
        for file in ${@}; do
          dir=files/${ns}/${pod}/${container}
          mkdir -p ${dir}
          kubectl cp $(basename ${pod}):${file} ${dir}/$(basename ${file}) -c ${container} -n ${ns}
        done
      fi
    done
  done
}

# Define the directory to store logs
dir=~/logs/$(hostname -s)
mkdir -p ${dir}
cd ${dir}

# Capture logs and resources from the Kubernetes cluster
non_namespaced operator.solo.hedera.com
for ns in solo-operator solo-orbit; do
  namespaced ${ns}
  logs ${ns}
done
files solo-orbit app.kubernetes.io/part-of=consensus-node consensus-node \
  /opt/hgcapp/services-hedera/HapiApp2.0/output/hgcaa.log \
  /opt/hgcapp/services-hedera/HapiApp2.0/output/swirlds.log
