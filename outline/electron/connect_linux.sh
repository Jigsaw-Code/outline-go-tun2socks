#!/bin/bash

# This script allows debugging outline-go-tun2socks directly on linux.
# Instructions:
# 1. Install the Outline client for Linux, connect to a server, and disconnect.
#    This installs the outline controller service.
# 2. $ ./connect_linux.sh <proxy IP or hostname> <proxy port> <password>
# 3. Ctrl+C to stop proxying

readonly PROXY_IP="$1"
readonly PROXY_PORT="$2"
readonly PROXY_PASSWORD="$3"

go build -v .

echo "{\"action\":\"configureRouting\",\"parameters\":{\"proxyIp\":\"${PROXY_IP}\",\"routerIp\":\"10.0.85.1\"}}" | socat UNIX-CONNECT:/var/run/outline_controller -
./electron -proxyHost "${PROXY_IP}" -proxyPort "${PROXY_PORT}" -proxyPassword "${PROXY_PASSWORD}" -logLevel debug -tunName outline-tun0
echo '{"action":"resetRouting","parameters":{}}' | socat UNIX-CONNECT:/var/run/outline_controller -
