#!/bin/bash
########################################
# Daily job script triggered by Prow.  #
########################################

WD=$(dirname $0)
WD=$(cd $WD; pwd)
ROOT=$(dirname $WD)
source prow/functions.sh

# Exit immediately for non zero status
set -e
# Check unset variables
set -u
# Print command trace
set -x

build_code
run_tests

create_images
create_manifest 

run_sonar_lint

# Run compliance only on Sunday
if [[ $(date +%w) == 0 ]]; then
	run_container_scan
fi

delete_images
exit 0
