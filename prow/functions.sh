# Common set of functions
# Error check is done with set -e command . Build will fail if any of the commands fail

# Variables expected from CI - PULL_NUMBER , JOB_TYPE , ARTIFACTS , SONAR_SCAN_TOKEN, SONARQUBE_URL, DOCKER_REGISTRY
DATE=$(date '+%Y%m%d')

print_step() {
	text_val=$1
	set +x
	echo " "
	echo "###################################################
#  ${text_val}	
###################################################"
	echo " "
	set -x
}

set_image_tag() {
	IMG_TAG="latest"
        IMG_PATH=""
	
	if [[ ${JOB_TYPE} == 'presubmit' ]]; then
	    VERSION_SUFFIX="-dev"
	    IMG_LOC='pr'	
	    IMG_TAG=${PULL_NUMBER}
	    PROD_BUILD_ID=${IMG_TAG}	
            IMG_PATH=spectro-images/${IMG_LOC}
        fi
	if [[ ${JOB_TYPE} == 'periodic' ]]; then
	    VERSION_SUFFIX="-$(date +%m%d%y)"
	    IMG_LOC='daily'	
	    IMG_TAG=$(date +%Y%m%d.%H%M)
	    PROD_BUILD_ID=${IMG_TAG}	
            IMG_PATH=spectro-images/${IMG_LOC}
	fi
	if [[ ${SPECTRO_RELEASE} ]] && [[ ${SPECTRO_RELEASE} == "yes" ]]; then
	    export VERSION_SUFFIX=""
	    IMG_LOC='release'	
	    IMG_TAG=$(make version)
	    PROD_BUILD_ID=$(date +%Y%m%d.%H%M)
            IMG_PATH=spectro-images-client/${IMG_LOC}
	    OVERLAY=overlays/release
	    DOCKER_REGISTRY=${DOCKER_REGISTRY_CLIENT}
	fi

	export PROD_BUILD_ID	
	export IMG_PATH
	export IMG_TAG 
	export VERSION_SUFFIX
	export PROD_VERSION=$(make version)
}

commenter() {
	export GITHUB_TOKEN=$ACCESS_TOKEN_PWD
	export GITHUB_OWNER=$REPO_OWNER
	export GITHUB_REPO=$REPO_NAME
	export GITHUB_COMMENT_TYPE=pr
	export GITHUB_PR_ISSUE_NUMBER=$PULL_NUMBER
	export GITHUB_COMMENT_FORMAT="Build logs for Job ${JOB_NAME} can be found here: {{.}}"
	export GITHUB_COMMENT="http://mayflower.spectrocloud.com/log?job=${JOB_NAME}&id=${BUILD_NUMBER}"
	github-commenter
}

set_release_vars() {
	RELEASE_DIR=gs://spectro-prow-artifacts/release/${REPO_NAME}
	VERSION_DIR=${RELEASE_DIR}/${PROD_VERSION}
	MARKER_FILE=marker
}

build_code() {
	print_step "Building Code"
	make all
}

run_tests() {
  print_step "Running Tests"
  make test
}

create_images() {
	print_step "Create and Push the images"
	make docker
}

delete_images() {
	print_step "Delete local images"
	make docker-rmi
}


create_manifest() {
	project_name=${REPO_NAME}
	print_step "Create manifest files and copy to artifacts folder"
	# Manifest output has all secrets printed. Mask the output
	make manifest > /dev/null 2>&1

	mkdir -p ${ARTIFACTS}/${project_name}/build
	cp -r config ${ARTIFACTS}/${project_name}/build/kustomize

	if [[ -d _build/manifests ]]; then
		cp -r _build/manifests ${ARTIFACTS}/manifests
	fi 
}

run_lint() {
	print_step "Running Lint check"
	golangci-lint run    ./...  --timeout 10m  --tests=false --skip-dirs tests --skip-dirs test
}


run_sonar_lint() {
	print_step "Running Lint check for Sonar Scanner"
	set +e 
	golangci-lint run    ./...  --out-format checkstyle --timeout 10m  --tests=false >  golangci-report.xml
	set -e
	if [[ -f golangci-report.xml ]]; then
		cat golangci-report.xml
	fi
}


run_sonar_scan() {
	print_step 'Run sonar-scanner for coverage and static code analysis'
	set +x
	if [[ -d _build/cov ]]; then
		cp -r _build/cov ${ARTIFACTS}/cov
	fi 
	/sonar/sonar-scanner/bin/sonar-scanner -Dsonar.projectKey=${REPO_NAME} -Dsonar.sources=. -Dsonar.host.url=${SONARQUBE_URL} -Dsonar.login=${SONAR_SCAN_TOKEN}
	set -x
}

#------------------------------------/
# Scan code for 3rd party licences   /
# Variables required are set in CI   /
#------------------------------------/
run_license_scan() {
	set +e 
	print_step 'Run license scan'
	COMPL_DIR=${ARTIFACTS}/compliance
	LICENSE_SCAN_DIR=${COMPL_DIR}/license_scan
	ISSUES_LIST=${LICENSE_SCAN_DIR}/issues.txt
	DEP_LIST=${LICENSE_SCAN_DIR}/dependencies.txt
	LIC_LIST=${LICENSE_SCAN_DIR}/licenses.txt
	mkdir -p ${LICENSE_SCAN_DIR}

	fossa init

	grep "project: ${REPO_NAME}" .fossa.yml
	if [[ $? -eq 0 ]]; then
		# This command will not work on MAC . Use a '' after -i in MAC
  		sed -i 's?project: \(.*\)?project: https://github.com/spectrocloud/\1?' .fossa.yml
	fi
	fossa analyze
	fossa test >> ${ISSUES_LIST}
	fossa report dependencies >> ${DEP_LIST}
	fossa report licenses >> ${LIC_LIST}

        gsutil cp -r $LIC_LIST gs://spectro-prow-artifacts/compliance/$DATE/source/license/${REPO_NAME}.txt
        gsutil cp -r $ISSUES_LIST gs://spectro-prow-artifacts/compliance/$DATE/source/sast/${REPO_NAME}.txt

	set -e 
}

#----------------------------------------------/
# Scan containers with Anchore and Trivy       /
# Variables required are set in CI             /
#----------------------------------------------/
run_container_scan() {
	set +e 
	print_step 'Run container scan'
	COMPL_DIR=${ARTIFACTS}/compliance
	CONTAINER_SCAN_DIR=${COMPL_DIR}/container_scan
	TRIVY_LIST=${CONTAINER_SCAN_DIR}/trivy_vulnerability.txt
	TRIVY_JSON=${CONTAINER_SCAN_DIR}/trivy_vulnerability.json
	mkdir -p ${CONTAINER_SCAN_DIR}
	
	for EACH_IMAGE in ${IMAGES_LIST}
	do
		trivy --download-db-only
 		echo "Image Name: ${EACH_IMAGE} " >> ${TRIVY_LIST}
		trivy ${EACH_IMAGE} >> ${TRIVY_LIST}
 	        trivy -f json ${EACH_IMAGE} >> ${TRIVY_JSON}
	done

	gsutil cp -r $TRIVY_LIST gs://spectro-prow-artifacts/compliance/$DATE/container/${REPO_NAME}.txt
	gsutil cp -r $TRIVY_JSON gs://spectro-prow-artifacts/compliance/$DATE/container/${REPO_NAME}.json
	set -e 
}

#----------------------------------------------/
# Check if the release has already been        /
# done with the same version                   /
#----------------------------------------------/
check_pre_released() {

	set +e 
	set_release_vars

	gsutil ls ${VERSION_DIR}/${MARKER_FILE} 
	if [[ $? -eq 0 ]]; then	
		echo "Version ${PROD_VERSION} has already been released and is available in release folder"
		exit 1
	fi
	set -e

}

#----------------------------------------------/
# Copy manifest files for this release         /
#  Also update the latest-version.txt file     /
#----------------------------------------------/
create_release_manifest() {

	print_step "Copy manifests to release folder"

	set_release_vars

	echo 'released'      > ${MARKER_FILE}

	gsutil cp -r config ${VERSION_DIR}/kustomize
	gsutil cp -r _build/manifests ${VERSION_DIR}/
	gsutil cp    ${MARKER_FILE}   ${VERSION_DIR}/

}


export REPO_NAME=cluster-api-provider-vsphere-static-ip
set_image_tag
export STATIC_IP_IMG=${DOCKER_REGISTRY}/${IMG_LOC}/capv-static-ip:${IMG_TAG}
IMAGES_LIST="${STATIC_IP_IMG}"
