#!/bin/bash

# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# This software is the property of WSO2 LLC. and its suppliers, if any.
# Dissemination of any information or reproduction of any material contained
# herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
# You may not alter or remove any copyright or other notice from copies of this content.
#

# Exit the script on any command with non-zero exit status.
set -e
set -o pipefail

# Check the number of arguments passed.
if [ "$#" -ne 2 ]; then
    echo "Error: Invalid or insufficient arguments provided!" >&2
    echo "Usage: $0 <GITHUB_TOKEN> <WORK_DIR>" >&2
    exit 1
fi

# Assign command line arguments to variables.
GIT_TOKEN=$1
WORK_DIR=$2
VERSION_TYPE=$3 # possible values: major, minor, patch

BUILD_DIRECTORY="$WORK_DIR/build"
RELEASE_DIRECTORY="$BUILD_DIRECTORY/releases"

# Configuration variables.
GIT_EMAIL="iam-cloud@wso2.com"
GIT_USERNAME="wso2-iam-cloud-bot"
UPSTREAM_REPO_URL="https://github.com/wso2/open-mcp-auth-proxy.git"
UPSTREAM_BRANCH="main"

# Configure git.
git config --global user.email "${GIT_EMAIL}"
git config --global user.name "${GIT_USERNAME}"

# Navigate to the working directory.
cd "${WORK_DIR}"

# Set 'origin' to point to the upstream repository.
git remote set-url origin "${UPSTREAM_REPO_URL}"

# Ensure the latest changes are pulled.
git checkout ${UPSTREAM_BRANCH}
git pull

# Create the release directory.
if [ ! -d "$RELEASE_DIRECTORY" ]; then
  mkdir -p "$RELEASE_DIRECTORY"
else
  rm -rf "$RELEASE_DIRECTORY"/*
fi

# Extract current version.
CURRENT_VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "1.0.0")
IFS='.' read -r MAJOR MINOR PATCH <<< "${CURRENT_VERSION}"

# Determine which part to increment
case "$VERSION_TYPE" in
  major)
    MAJOR=$((MAJOR + 1))
    MINOR=0
    PATCH=0
    ;;
  minor)
    MINOR=$((MINOR + 1))
    PATCH=0
    ;;
  patch|*)
    PATCH=$((PATCH + 1))
    ;;
esac

NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"

echo "Creating release packages for version $NEW_VERSION..."

# List of supported OSes.
oses=("linux" "linux-arm" "darwin")

# Navigate to the release directory.
cd "${RELEASE_DIRECTORY}"

for os in "${oses[@]}"; do
  os_dir="../$os"
  
  if [ -d "$os_dir" ]; then
    release_artifact_folder="openmcpauthproxy_${os}-v${CURRENT_VERSION}"
    mkdir -p "$release_artifact_folder"
    cp -r "$os_dir/*" "$release_artifact_folder"

    # Zip the release package.
    zip_file="$release_artifact_folder.zip"
    echo "Creating $zip_file..."
    zip -r "$zip_file" "$release_artifact_folder"

    # Delete the folder after zipping.
    rm -rf "$release_artifact_folder"

    # Generate checksum file.
    sha256sum "$zip_file" | sed "s|target/releases/||" > "$zip_file.sha256"
    echo "Checksum generated for the $os package."

    echo "Release packages created successfully for $os."
  else
    echo "Skipping $os release package creation as the build artifacts are not available."
  fi
done

echo "Release packages created successfully in $RELEASE_DIRECTORY."

# Navigate back to the project root directory.
cd "${WORK_DIR}"

# Collect all ZIP and .sha256 files in the target/releases directory.
FILES_TO_UPLOAD=$(find build/releases -type f \( -name "*.zip" -o -name "*.sha256" \))

# Create a release with the current version.
TAG_NAME="v${NEW_VERSION}"
export GITHUB_TOKEN="${GIT_TOKEN}"
gh release create "${TAG_NAME}" ${FILES_TO_UPLOAD} --title "${TAG_NAME}" --notes "OpenMCPAuthProxy - ${TAG_NAME}" --target "${UPSTREAM_BRANCH}" || { echo "Failed to create release"; exit 1; }


echo "Release ${TAG_NAME} created successfully."
