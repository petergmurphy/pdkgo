#!/usr/bin/env pwsh

# Set goreleaser to build for current platform only
goreleaser build --snapshot --rm-dist --single-target
