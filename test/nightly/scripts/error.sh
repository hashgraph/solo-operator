#! /usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0

# Enable error handling and define a custom error handler
set -E

# Function to handle errors and print debug information
error_handler() {
  exit_code=${?}
  restore_xtrace=$(set +o | grep xtrace)
  set +x
  echo "Error(${exit_code}): ${BASH_COMMAND}"
  for ((i = 1; i < ${#BASH_LINENO[@]}; i++)); do
      echo "  at ${FUNCNAME[i]} (${BASH_SOURCE[i]}:${BASH_LINENO[i-1]})"
  done
  ${restore_xtrace}
}

# Set a trap to call the error handler on any error
trap error_handler ERR
