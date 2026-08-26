#!/bin/sh
set -eu

docker build -f benzhi.Dockerfile -t fleetforge-benzhi:go1.23.12 .
