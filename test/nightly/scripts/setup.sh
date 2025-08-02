#! /usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# Exit immediately if a command exits with a non-zero status
set -e
set -o pipefail

# Get the directory of the current script
bin=$(dirname "$(readlink -f -- "${BASH_SOURCE}")")

. ${bin}/error.sh

# Capture the version to upgrade from and to
from_version=${1}
to_version=${2}

# Determine the OS release file to source
test -e /etc/os-release && os_release='/etc/os-release' || os_release='/usr/lib/os-release'
. "${os_release}"

# Execute OS-specific setup steps based on the detected OS
case ${ID} in
    rhel) ${bin}/rhel-8-9-steps.sh ;;
    ubuntu) ${bin}/ubuntu-focal-jammy-steps.sh ;;
esac

# Clone the repository and build the yahcli tool
git clone --depth 1 --branch v${from_version} https://github.com/hiero-ledger/hiero-consensus-node.git
pushd hiero-consensus-node
./gradlew -version
# ./gradlew clean yahcli:assemble
# ln -s $(realpath hedera-node/yahcli/build/libs/yahcli-*-shadow.jar) ~/yahcli.jar
./gradlew clean test-clients:yahCliJar --console plain --info
ln -s $(realpath hedera-node/test-clients/build/libs/test-clients-*-yahcli.jar) ~/yahcli.jar
ln -s $(realpath hedera-node/test-clients/yahcli/localhost) ~
popd

# Prepare for the software upgrade
mkdir -p ~/localhost/sysfiles
wget -O ~/localhost/sysfiles/softwareUpgrade.zip https://builds.hedera.com/node/software/v${to_version%.*}/build-v${to_version}.zip
zip -d ~/localhost/sysfiles/softwareUpgrade.zip '*.jar'
