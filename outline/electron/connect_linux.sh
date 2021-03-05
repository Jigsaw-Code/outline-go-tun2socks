#!/bin/bash

# This script allows debugging outline-go-tun2socks directly on linux.
# Instructions:
# 1. Install the Outline client for Linux, connect to a server, and disconnect.
#    This installs the outline controller service.
# 2. $ git update-index --assume-unchanged connect_linux.sh
#    This helps to avoid accidentally checking in your proxy credentials.
# 3. Edit this script to add the IP, port, and password for your test proxy.
# 4. $ ./connect_linux.sh
# 5. Ctrl+C to stop proxying

readonly PROXY_IP="..."
readonly PROXY_PORT="..."
readonly PROXY_PASSWORD="..."

go build -v .

echo "{\"action\":\"configureRouting\",\"parameters\":{\"proxyIp\":\"${PROXY_IP}\",\"routerIp\":\"10.0.85.1\"}}" | socat UNIX-CONNECT:/var/run/outline_controller -
./electron -proxyHost "${PROXY_IP}" -proxyPort "${PROXY_PORT}" -proxyPassword "${PROXY_PASSWORD}" -logLevel debug -tunName outline-tun0
echo '{"action":"resetRouting","parameters":{}}' | socat UNIX-CONNECT:/var/run/outline_controller -
