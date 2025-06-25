#! /usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# Exit immediately if a command exits with a non-zero status
set -e
set -o pipefail

# Set default values for environment variables if not already defined
export LOCAL_DOWNLOAD_DIR=${CACHE_DIR:-cache_dir}
export REMOTE_DOWNLOAD_DIR=/tmp/provisioner
export HOSTNAME_PREFIX=${HOSTNAME_PREFIX:-local-}
export MACHINE=${MACHINE:-n4-highmem-4}
export SSH_OPTIONS="-i ~/.ssh/google_compute_engine -o StrictHostKeyChecking=no -o ServerAliveInterval=60 -o ServerAliveCountMax=5"
export USER=${USER:-$(id -un)}
export VERSION=${VERSION:-0.0.0-$(git rev-parse @)}
export VERSION_A=${VERSION_A:-0.59.0}
export VERSION_B=${VERSION_B:-0.59.5}
export ZONE=${ZONE:-us-central1-a}

# Source the error handling script
. $(dirname "$(readlink -f -- "${BASH_SOURCE}")")/error.sh

# Function to handle cleanup and log capture on script exit
exit_handler() {
  run_on_nodes "./test/scripts/capture-logs.sh > ~/logs/\$(hostname -s)/capture-logs.log 2>&1"
  scp_from_nodes logs .
  run delete_vms
}

# Set a trap to call the exit handler on script exit
trap exit_handler EXIT

# Function to run a command and group its output
run() {
  echo "::group::${1}"
  ${@}
  echo "::endgroup::"
}

# Function to create a VM instance
create_vm() {
  name="${HOSTNAME_PREFIX}${1}"
  image_project=${2}
  image_family=${3}
  gcloud compute instances create ${name} --zone=${ZONE} --boot-disk-size=64GB \
    --machine-type=${MACHINE} --image-project=${image_project} --image-family=${image_family}
}

# Function to create multiple VM instances
create_vms() {
  create_vm node1 rhel-cloud rhel-8
  create_vm node2 rhel-cloud rhel-9
  create_vm node3 ubuntu-os-pro-cloud ubuntu-pro-2004-lts
  create_vm node4 ubuntu-os-cloud ubuntu-2204-lts
}

# Function to list VM instances with the specified prefix
list_instances() {
  gcloud compute instances list "${@}" | grep "^${HOSTNAME_PREFIX}" || true
}

# Function to list VM instances without the prefix
list_instances_without_prefix() {
  list_instances "${@}" | sed 's,^\S*-,,'
}

# Function to delete all VMs with the specified prefix
delete_vms() {
  list_instances --format='value(name)' | xargs -r gcloud compute instances delete --quiet --zone=${ZONE}
}

# Function to run a command on all nodes
run_on_nodes() {
  echo ${xip[@]} | tr ' ' '\n' | parallel -j ${#xip[@]} --quote ssh ${SSH_OPTIONS} ${USER}@{} ${@}
}

# Function to run a command on a specific node
run_on() {
  ip=${xip[${1}]}
  shift
  ssh ${SSH_OPTIONS} ${USER}@${ip} ${@}
}

# Function to copy files to all nodes
scp_to_nodes() {
  ARGS=(${@})
  echo ${xip[@]} | tr ' ' '\n' | parallel -j ${#xip[@]} scp ${SSH_OPTIONS} -pqr "${ARGS[@]:0:$((${#ARGS[@]}-1))}" "${USER}@{}:${ARGS[@]: -1:1}"
}

# Function to copy files from all nodes
scp_from_nodes() {
  ARGS=(${@})
  echo ${xip[@]} | tr ' ' '\n' | parallel -j ${#xip[@]} scp ${SSH_OPTIONS} -pqr $(printf "${USER}@{}:%s " "${ARGS[@]:0:$((${#ARGS[@]}-1))}") "${ARGS[@]: -1:1}"
}

# Function to copy files from a specific node
scp_from_node() {
  ip=${xip[${1}]}
  shift
  ARGS=(${@})
  scp ${SSH_OPTIONS} -pqr $(printf "${USER}@${ip}:%s " "${ARGS[@]:0:$((${#ARGS[@]}-1))}") "${ARGS[@]: -1:1}"
}

# Function to verify SSH connectivity to a VM
verify_ssh() {
  name=${1}
  for ((i=0; i<10; i++)); do
    if gcloud compute ssh --quiet --zone=${ZONE} ${name} --command=:; then return; fi
    sleep 15
  done
  return 1
}

# Function to set up SSH for all VMs
setup_ssh() {
  for name in $(list_instances --format='value(name)'); do
    verify_ssh ${name}
  done
}

# Function to update files with environment variables
update_files() {
  . <(list_instances_without_prefix --format='value[separator="_ip="](name, INTERNAL_IP)' | tr a-z A-Z | sed 's,^,export ,')
  export LAYER_HASHES=$(skopeo inspect --override-os=linux --override-arch=amd64 docker://swirldslabs.jfrog.io/consensus-node-docker-release/consensus-node:${VERSION_B} | jq -c .Layers)
  REPLACE="$(env | cut -d= -f1 | grep -x -e NODE._IP -e VERSION_. -e LAYER_HASHES | sed -z 's/\(\S*\)\n/${\1},/g')"
  for file in $(git ls-files test/nightly/*.yaml); do envsubst "${REPLACE}" < ${file} | sponge ${file}; done
}

# Function to set up nodes
setup_nodes() {
  scp_to_nodes test/nightly test
  for node in ${!xip[@]}; do
    if [ -d "${LOCAL_DOWNLOAD_DIR}/${node}" ]; then
      tar -C "${LOCAL_DOWNLOAD_DIR}/${node}" -cf - . | run_on ${node} "mkdir -p ${REMOTE_DOWNLOAD_DIR} && tar -C ${REMOTE_DOWNLOAD_DIR} -xf -"
    fi
  done
  run_on_nodes "mkdir -p ~/logs/\$(hostname -s)"
  run_on_nodes "./test/scripts/setup.sh ${VERSION_A} ${VERSION_B} 2>&1 | tee ~/logs/\$(hostname -s)/setup.log"
}

# Function to build the operator
build_operator() {
  ./gradlew clean assemble -Pversion=${VERSION} -Dquarkus.container-image.build=true -Dquarkus.container-image.registry=localhost -Dquarkus.container-image.group=hashgraph
  docker save localhost/hashgraph/solo-operator:${VERSION} -o container.tar
  sed -i 's,imagePullPolicy: "Always",imagePullPolicy: "Never",' build/helm/templates/deployment.yaml
  helm package build/helm
}

# Function to install the operator
install_operator() {
  scp_to_nodes container.tar solo-operator-${VERSION}.tgz .
  run_on_nodes sudo /usr/local/bin/ctr -n=k8s.io images import container.tar
  run_on_nodes helm upgrade --install solo-operator solo-operator-${VERSION}.tgz -n solo-operator --create-namespace
  run_on_nodes kubectl wait -n solo-operator --for=condition=Ready --timeout 1m pods -l app.kubernetes.io/name=solo-operator
}

# Function to execute the genesis process
genesis() {
  run_on_nodes test/scripts/genesis.sh
}

# Function to perform a network upgrade
network_upgrade() {
  node=node$((RANDOM % ${#xip[@]} + 1))

  run_on ${node} java -jar yahcli.jar -c test/yahcli/config.yaml -n localhost -p 2 activate-staking -b 1000h -p 100h -r 5h
  sleep 90
  run_on ${node} java -jar yahcli.jar -c test/yahcli/config.yaml -n localhost -p 2 sysfiles upload software-zip
  sleep 10
  run_on_nodes "kubectl logs -n solo-orbit -l app.kubernetes.io/part-of=consensus-node -c consensus-node --tail -1 | grep stake=10000000000,"
  run_on ${node} "java -jar yahcli.jar -c test/yahcli/config.yaml -n localhost -p 2 prepare-upgrade -f 150 -h \$(sha384sum localhost/sysfiles/softwareUpgrade.zip | cut -d ' ' -f1)"
  run_on_nodes kubectl apply -f test/network-operation/network-upgrade-execute-immediate.yaml
  run_on_nodes kubectl wait -n solo-orbit --for=condition=ImageVerified=True networkoperations.operator.solo.hedera.com/network-upgrade --timeout=5m
  run_on ${node} "java -jar yahcli.jar -c test/yahcli/config.yaml -n localhost -p 2 freeze-upgrade -f 150 -h \$(sha384sum localhost/sysfiles/softwareUpgrade.zip | cut -d ' ' -f1) -s \$(date --date='5 minutes' -u +%F.%T)"
  run_on_nodes kubectl apply -f test/network-operation/network-upgrade-freeze-scheduled.yaml
  run_on_nodes kubectl wait -n solo-orbit pods -l app.kubernetes.io/part-of=consensus-node --for=condition=Ready=False --timeout=10m
  run_on_nodes kubectl apply -f test/network-operation/network-upgrade-now-frozen.yaml
  run_on_nodes kubectl wait -n solo-orbit --for=jsonpath=status.phase=Succeeded --timeout=10m networkoperations.operator.solo.hedera.com/network-upgrade
  run_on_nodes kubectl wait -n solo-orbit pods -l app.kubernetes.io/part-of=consensus-node --for=condition=Ready --timeout=10m
}

# Function to save downloaded files from nodes
save_downloads() {
  for node in ${!xip[@]}; do
    mkdir -p "${LOCAL_DOWNLOAD_DIR}/${node}"
    scp_from_node ${node} "${REMOTE_DOWNLOAD_DIR}/*" "${LOCAL_DOWNLOAD_DIR}/${node}/"
  done
}

# Main script execution
run delete_vms
run create_vms
declare -A xip
. <(list_instances_without_prefix --format="value('name', 'EXTERNAL_IP')" | awk '{printf("xip[%s]=%s\n", $1, $2);}')
run setup_ssh
run update_files
run setup_nodes
run build_operator
run install_operator
run genesis
run network_upgrade
run save_downloads
echo Test passed successfully
