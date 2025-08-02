#! /usr/bin/env sh

set -eu

host=127.0.0.1
port=9999
required_ports="9999 50111 50211 50212"

# Check listening ports
awk -v ports="$required_ports" '
  BEGIN {
    split(ports, arr)
    for (i in arr) required[arr[i]] = 1
  }
  FNR != 1 {
    split($2, arr, ":")
    listening[sprintf("%d", "0x" arr[2])] = 1
  }
  END {
    for (p in listening) { delete required[p] }
    if (length(required)) {
      print "service not ready"
      exit 1
    }
  }
' /proc/net/tcp /proc/net/tcp6

# Make HTTP request and parse response
printf "GET / HTTP/1.1\r\nConnection: close\r\n\r\n" |
  timeout 1 nc "$host" "$port" |
  awk '
    BEGIN { in_header = 1 }
    NR == 1 && $2 != 200 { print "HTTP status "$2" unexpected"; exit 1 }
    /^\r$/ { in_header=0; next }
    in_header { next }
  
    # # HELP platform_PlatformStatus 0=NO_VALUE 1=STARTING_UP 2=ACTIVE ...
    /^# HELP platform_PlatformStatus / { help = $0 }
  
    # platform_PlatformStatus{node="0",} 2.0
    /^platform_PlatformStatus{/ { code = int($NF) }
  
    END {
      status = "UNKNOWN"
      pattern = "[[:space:]]" code "=([^[:space:]]+)"
      if (match(help, pattern)) {
        str = substr(help, RSTART, RLENGTH)
        status = substr(str, index(str, "=") + 1)
      }
      if (status != "ACTIVE") {
        printf("status is %s (%s)\n", code, status)
        exit 1
      }
    }
  '
