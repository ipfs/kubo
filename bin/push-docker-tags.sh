#!/usr/bin/env bash
#
# TODO: this script is legacy, use get-docker-tags.sh instead.
#
# push-docker-tags.sh
#
# Run from ci to tag images based on the current branch or tag name.
# A bit like dockerhub autobuild config, but somewhere we can version control it.
#
# The `docker-build` job builds the current commit in docker and tags it as ipfs/go-ipfs:wip
#
# Then the `docker-publish` job runs this script to decide what tag, if any,
# to publish to dockerhub.
#
# Usage:
#   ./push-docker-tags.sh <build number> <git commit sha1> <git branch name> [git tag name] [dry run]
#
# Example:
#   # dry run. pass a 5th arg to have it print what it would do rather than do it.
#   ./push-docker-tags.sh $(date -u +%F) testingsha master "" dryrun
#
#   # push tag for the master branch
#   ./push-docker-tags.sh $(date -u +%F) testingsha master
#
#   # push tag for a release tag
#   ./push-docker-tags.sh $(date -u +%F) testingsha release v0.5.0
#
set -euo pipefail

if [[ $# -lt 1 ]] ; then
  echo 'At least 1 arg required. Pass 5 args for a dry run.'
  echo 'Usage:'
  echo './push-docker-tags.sh <build number> [git commit sha1] [git branch name] [git tag name] [dry run]'
  exit 1
fi

BUILD_NUM=$1
GIT_SHA1=${2:-$(git rev-parse HEAD)}
GIT_SHA1_SHORT=$(echo "$GIT_SHA1" | cut -c 1-7)
GIT_BRANCH=${3:-$(git symbolic-ref -q --short HEAD || echo "unknown")}
GIT_TAG=${4:-$(git describe --tags --exact-match || echo "")}
DRY_RUN=${5:-false}

WIP_IMAGE_TAG=${WIP_IMAGE_TAG:-wip}
IMAGE_NAME=${IMAGE_NAME:-ipfs/go-ipfs}

pushTag () {
  local IMAGE_TAG=$1
  if [ "$DRY_RUN" != false ]; then
    echo "DRY RUN! I would have tagged and pushed the following..."
    echo docker tag "$IMAGE_NAME:$WIP_IMAGE_TAG" "$IMAGE_NAME:$IMAGE_TAG"
    echo docker push "$IMAGE_NAME:$IMAGE_TAG"
  else
    echo "Tagging $IMAGE_NAME:$IMAGE_TAG and pushing to dockerhub"
    docker tag "$IMAGE_NAME:$WIP_IMAGE_TAG" "$IMAGE_NAME:$IMAGE_TAG"
    docker push "$IMAGE_NAME:$IMAGE_TAG"
  fi
}

if [[ $GIT_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+-rc ]]; then
  pushTag "$GIT_TAG"

elif [[ $GIT_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  pushTag "$GIT_TAG"
  pushTag "latest"
  pushTag "release" # see: https://github.com/ipfs/go-ipfs/issues/3999#issuecomment-742228981

elif [[ $GIT_BRANCH =~ ^bifrost-.* ]]; then
  # sanitize the branch name since docker tags have stricter char limits than git branch names
  branch=$(echo "$GIT_BRANCH" | tr '/' '-' | tr --delete --complement '[:alnum:]-')
  pushTag "${branch}-${BUILD_NUM}-${GIT_SHA1_SHORT}"

elif [ "$GIT_BRANCH" = "master" ] || [ "$GIT_BRANCH" = "staging" ]; then
  pushTag "${GIT_BRANCH}-${BUILD_NUM}-${GIT_SHA1_SHORT}"
  pushTag "${GIT_BRANCH}-latest"

else
  echo "Nothing to do. No docker tag defined for branch: $GIT_BRANCH, tag: $GIT_TAG"

fi
