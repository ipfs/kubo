#!/usr/bin/env bash

# get-docker-tags.sh
#
# Usage:
#   ./get-docker-tags.sh <build number> <git commit sha1> <git branch name> [git tag name]
#
# Example:
#
#   # get tag for the master branch
#   ./get-docker-tags.sh $(date -u +%F) testingsha master
#
#   # get tag for a release tag
#   ./get-docker-tags.sh $(date -u +%F) testingsha release v0.5.0
#
set -euo pipefail

if [[ $# -lt 1 ]] ; then
  echo 'At least 1 arg required.'
  echo 'Usage:'
  echo './get-docker-tags.sh <build number> [git commit sha1] [git branch name] [git tag name]'
  exit 1
fi

BUILD_NUM=$1
GIT_SHA1=${2:-$(git rev-parse HEAD)}
GIT_SHA1_SHORT=$(echo "$GIT_SHA1" | cut -c 1-7)
GIT_BRANCH=${3:-$(git symbolic-ref -q --short HEAD || echo "unknown")}
GIT_TAG=${4:-$(git describe --tags --exact-match 2> /dev/null || echo "")}

IMAGE_NAME=${IMAGE_NAME:-ipfs/kubo}
LEGACY_IMAGE_NAME=${LEGACY_IMAGE_NAME:-ipfs/go-ipfs}

echoImageName () {
  local IMAGE_TAG=$1
  echo "$IMAGE_NAME:$IMAGE_TAG"
  echo "$LEGACY_IMAGE_NAME:$IMAGE_TAG"
}

if [[ $GIT_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+-rc ]]; then
  echoImageName "$GIT_TAG"

elif [[ $GIT_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echoImageName "$GIT_TAG"
  echoImageName "latest"
  echoImageName "release" # see: https://github.com/ipfs/go-ipfs/issues/3999#issuecomment-742228981

elif [[ $GIT_BRANCH =~ ^bifrost-.* ]]; then
  # sanitize the branch name since docker tags have stricter char limits than git branch names
  branch=$(echo "$GIT_BRANCH" | tr '/' '-' | tr --delete --complement '[:alnum:]-')
  echoImageName "${branch}-${BUILD_NUM}-${GIT_SHA1_SHORT}"

elif [ "$GIT_BRANCH" = "master" ] || [ "$GIT_BRANCH" = "staging" ]; then
  echoImageName "${GIT_BRANCH}-${BUILD_NUM}-${GIT_SHA1_SHORT}"
  echoImageName "${GIT_BRANCH}-latest"

else
  echo "Nothing to do. No docker tag defined for branch: $GIT_BRANCH, tag: $GIT_TAG"

fi
