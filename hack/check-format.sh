#!/bin/bash
set -eu

DIFF=$( gofmt -s -d cmd pkg )
if [ -n "${DIFF}" ]; then
	echo "${DIFF}"
	exit 1
fi
exit 0
