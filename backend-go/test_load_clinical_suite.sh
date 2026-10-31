#!/usr/bin/env bash

# ============================================================================
# SATUSEHAT STG - BASH CLINICAL SUITE LOAD TEST
# ============================================================================
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
node "$DIR/test_load_clinical_suite.js"
