#!/bin/sh
set -eu
cd "$(dirname "$0")"
go build ./...
npm run build
