<!-- Last updated by @galargh during [X.Y.Z release](https://github.com/ipfs/kubo/issues/9319) -->

> Release Issue Template.  If doing a patch release, see [here](https://github.com/ipfs/kubo/blob/master/docs/PATCH_RELEASE_TEMPLATE.md)

# Items to do upon creating the release issue
- [ ] Fill in the Meta section
- [ ] Assign the issue to the release owner and reviewer.
- [ ] Name the issue "Release vX.Y.Z"
- [ ] Set the proper values for X.Y.Z
- [ ] Pin the issue

# Meta
* Release owner: @who
* Release reviewer: @who
* Expected RC date: week of YYYY-MM-DD
* 🚢 Expected final release date: YYYY-MM-DD
* Accompanying PR for improving the release process: (example: https://github.com/ipfs/kubo/pull/9391)

See the [Kubo release process](https://pl-strflt.notion.site/Kubo-Release-Process-5a5d066264704009a28a79cff93062c4) for more info.

# Kubo X.Y.Z Release

We're happy to announce Kubo X.Y.Z!

As usual, this release includes important fixes, some of which may be critical for security. Unless the fix addresses a bug being exploited in the wild, the fix will _not_ be called out in the release notes. Please make sure to update ASAP. See our [security fix policy](https://github.com/ipfs/go-ipfs/tree/master/docs/releases.md#security-fix-policy) for details.

## 🗺 What's left for release

<List of items with PRs and/or Issues to be considered for this release>

### Required

### Nice to have

## 🔦 Highlights

< top highlights for this release notes. For ANY version (final or RCs) >

## ✅ Release Checklist

Checklist:

- [ ] **Stage 0 - Prerequisites**
  - [ ] Open an issue against [bifrost-infra](https://github.com/protocol/bifrost-infra) ahead of the release ([example](https://github.com/protocol/bifrost-infra/issues/2109)).  **Idealy, do this multiple days in advance of the RC** to give Bifrost the heads up that asks will be coming their way.
    - [ ] Spell out all that we want updated - gateways, the bootstraper and the cluster/preload nodes
    - [ ] Mention @protocol/bifrost-team in the issue and let them know the expected date of the release
      - Issue link:
        <details>
          # create new issue in protocol/bifrost-infra
          gh api \
            --method POST \
            --raw-field "title=Rollout Kubo vX.Y.Z-RCN" \
            --raw-field "body=## What should be updated

            - [ ] Gateways
            - [ ] Bootstrapper
            - [ ] Cluster/Preload nodes

            ## When

            YYYY-MM-DD" \
            repos/protocol/bifrost-infra/issues
        </details>
  - [ ] Ensure that the `What's left for release` section has all the checkboxes checked. If that's not the case, discuss the open items with Kubo maintainers and update the release schedule accordingly.
  - [ ] Create `docs-release-vX.Y.Z` branch, open a draft PR and keep updating `docs/RELEASE_ISSUE_TEMPLATE.md` on that branch as you go.
      - [ ] Link it in the "Meta" section above.
        <details>
          # retrieve master ref for ipfs/kubo
          gh api /repos/ipfs/kubo/git/ref/heads/master

          # create docs-release-vX.Y.Z ref for ipfs/kubo
          gh api \
            --method POST \
            /repos/ipfs/kubo/git/refs \
            -f ref='refs/heads/docs-release-vX.Y.Z' \
            -f sha='254d81a9d5595c3e637c7573d56125836d5f5055'

          # create draft PR from docs-release-vX.Y.Z to master for ipfs/kubo
          # requires docs-release-vX.Y.Z to be modified
          gh api \
            --method POST \
            /repos/ipfs/kubo/pulls \
            -f title='docs: release vX.Y.Z' \
            -f head='docs-release-vX.Y.Z' \
            -f base='master' \
            -f draft=true
        </details>
  - [ ] Ensure you have a [GPG key generated](https://docs.github.com/en/authentication/managing-commit-signature-verification/generating-a-new-gpg-key) and [added to your GitHub account](https://docs.github.com/en/authentication/managing-commit-signature-verification/adding-a-gpg-key-to-your-github-account). This will enable you to created signed tags.
  - [ ] Ensure you have [admin access](https://discuss.ipfs.tech/g/admins) to [IPFS Discourse](https://discuss.ipfs.tech/). Admin access is required to globally pin posts and create banners. @2color might be able to assist you.
  - [ ] Access to [#bifrost](https://filecoinproject.slack.com/archives/C03MMMF606T) channel in FIL Slack might come in handy. Ask the release reviewer to invite you over.
  - [ ] Access to [#shared-pl-marketing-requests](https://filecoinproject.slack.com/archives/C018EJ8LWH1) channel in FIL Slack will be required to request social shares. Ask the release reviewer to invite you over.
  - [ ] After the release is deployed to our internal infrastructure, you're going to need read access to [IPFS network metrics](https://github.com/protocol/pldw/blob/624f47cf4ec14ad2cec6adf601a9f7b203ef770d/docs/sources/ipfs.md#ipfs-network-metrics) dashboards. Open an access request in https://github.com/protocol/pldw/issues/new/choose if you don't have it yet ([example](https://github.com/protocol/pldw/issues/158)).
  - [ ] You're also going to need NPM installed on your system. See [here](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm) for instructions.
  - [ ] Prepare changelog proposal in [docs/changelogs/vX.Y.md](https://github.com/ipfs/kubo/blob/master/docs/changelogs/).
    - Skip filling out the `### Changelog` section (the one where which lists all the commits and contributors) for now. We will populate it after the release branch is cut.
    - PR link:
  - [ ] Install ZSH ([instructions](https://github.com/ohmyzsh/ohmyzsh/wiki/Installing-ZSH#install-and-set-up-zsh-as-default)). It is needed by the changelog creation script.
  - [ ] Ensure you have `kubo` checked out under `$(go env GOPATH)/src/github.com/ipfs/kubo`. This is required by the changelog creation script.
    - If you want your clone to live in a different location, you can symlink it to the expected location by running `mkdir -p $(go env GOPATH)/src/github.com/ipfs && ln -s $(pwd) $(go env GOPATH)/src/github.com/ipfs/kubo`.
  - [ ] Ensure that [README.md](https://github.com/ipfs/go-ipfs/tree/master/README.md) is up to date.
- [ ] **Stage 1 - Initial Preparations**
  - [ ] Upgrade to the latest patch release of Go that CircleCI has published (currently used version: `1.19.1`)
    - [ ] See the list here: https://hub.docker.com/r/cimg/go/tags
          <details>
          # retrieve the latest version of cimg/go available
          curl -s 'https://hub.docker.com/v2/repositories/cimg/go/tags' | jq -r '.results | map(.name) | map(select(. | test("^[0-9]+\\.[0-9]+\\.[0-9]+$"))) | .[0]'
          </details>
    - [ ] [ipfs/distributions](https://github.com/ipfs/distributions): bump [this version](https://github.com/ipfs/distributions/blob/master/.tool-versions#L2)
          <details>
          # checkout new branch
          git checkout -b bump-go-version

          # replace the cimg/go version in .tool-versions in ipfs/distributions
          sed -i 's/golang [0-9]\+\.[0-9]\+\.[0-9]\+/golang 1.19.3/g' .tool-versions

          # commit the change
          git add .tool-versions
          git commit -m "chore: bump go version to 1.19.3"

          # push the change
          git push origin bump-go-version

          # open a PR
          gh api /repos/ipfs/distributions/pulls \
            --method POST \
            -f title='chore: bump go version to 1.19.3' \
            -f head='bump-go-version' \
            -f base='master'
          </details>
    - [ ] [ipfs/kubo](https://github.com/ipfs/kubo): [example PR](https://github.com/ipfs/kubo/pull/8599)
          <details>
          # checkout new branch
          git checkout -b bump-go-version

          # replace the cimg/go version in .circleci/main.yml in ipfs/kubo
          sed -i 's/cimg\/go:[0-9]\+\.[0-9]\+\.[0-9]\+/cimg\/go:1.19.3/g' .circleci/main.yml

          # replace golang version in Dockerfile
          sed -i 's/golang:[0-9]\+\.[0-9]\+\.[0-9]\+/golang:1.19.3/g' Dockerfile

          # commit the change
          git add .circleci/main.yml Dockerfile
          git commit -m "chore: bump go version to 1.19.3"

          # push the change
          git push origin bump-go-version

          # open a PR
          gh api /repos/ipfs/kubo/pulls \
            --method POST \
            -f title='chore: bump go version to 1.19.3' \
            -f head='bump-go-version' \
            -f base='master'
          </details>
    - [ ] [ipfs/ipfs-docs](https://github.com/ipfs/ipfs-docs): [example PR](https://github.com/ipfs/ipfs-docs/pull/1298) - only if the major version changed
  - [ ] Fork a new branch (`release-vX.Y.Z`) from `master`.
        <details>
        # retrieve master ref for ipfs/kubo
        gh api /repos/ipfs/kubo/git/ref/heads/master

        # create release-vX.Y.Z ref for ipfs/kubo
        gh api \
          --method POST \
          /repos/ipfs/kubo/git/refs \
          -f ref='refs/heads/release-vX.Y.Z' \
          -f sha='a4da8f6cc768c3e2cce9c2677a792b2c237066aa'
        </details>
  - [ ] Bump the version in `version.go` in the `master` branch to `vX.(Y+1).0-dev` via a PR ([example](https://github.com/ipfs/kubo/pull/9305)).
        <details>
        # checkout new branch
        git checkout -b bump-version

        # replace the version in version.go
        sed -i 's/const CurrentVersionNumber = ".*"/const CurrentVersionNumber = "0.18.0-dev"/g' version.go

        # commit the change
        git add version.go
        git commit -m "chore: bump version to v0.18.0-dev"

        # push the change
        git push origin bump-version

        # open a PR
        gh api /repos/ipfs/kubo/pulls \
          --method POST \
          -f title='chore: bump version to v0.18.0-dev' \
          -f head='bump-version' \
          -f base='master'
        </details>
- [ ] **Stage 2 - Release Candidate** - _if any [non-trivial](docs/releases.md#footnotes) changes need to be included in the release, return to this stage_
  - [ ] If it's not a first RC, add new commits to the `release-vX.Y.Z` branch from `master` using `git cherry-pick -x ...`
      - Note: `release-*` branches are protected. You can do all needed updates on a separated branch (e.g. `wip-release-vX.Y.Z`) and when everything is settled push to `release-vX.Y.Z`
  - [ ] Bump the version in `version.go` in the `release-vX.Y.Z` branch to `vX.Y.Z-rcN`.
        <details>
        # checkout new branch
        git checkout -B bump-release-version

        # replace the version in version.go
        sed -i 's/const CurrentVersionNumber = ".*"/const CurrentVersionNumber = "X.Y.Z-rcN"/g' version.go

        # commit the change
        git add version.go
        git commit -m "chore: bump version to vX.Y.Z-rcN"

        # push the change
        git push origin bump-release-version

        # open a PR
        gh api /repos/ipfs/kubo/pulls \
          --method POST \
          -f title='chore: bump version to vX.Y.Z-rcN' \
          -f head='bump-release-version' \
          -f base='release-vX.Y.Z'
        </details>
  - [ ] If it's a first RC, create a draft PR targetting `release` branch if it doesn't exist yet ([example](https://github.com/ipfs/kubo/pull/9306)).
        <details>
        # open a PR
        gh api /repos/ipfs/kubo/pulls \
          --method POST \
          -f title='wip: release vX.Y.Z' \
          -f head='release-vX.Y.Z' \
          -f base='release' \
          -f draft=true
        </details>
  - [ ] Wait for CI to run and complete PR checks. All checks should pass.
  - [ ] Create a signed tag for the release candidate.
    - [ ] This is a dangerous operation, as it is difficult to reverse due to Go modules and automated Docker image publishing. Remember to verify the commands you intend to run for items marked with ⚠️ with the release reviewer.
    - [ ] ⚠️ Tag HEAD `release-vX.Y.Z` commit with `vX.Y.Z-rcN` (`git tag -s vX.Y.Z-rcN -m 'Pre-release X.Y.Z-rcn'`)
        <details>
        # create a signed tag
        git tag -s vX.Y.Z-rcN -m 'Pre-release X.Y.Z-rcN'
        </details>
    - [ ] Run `git show vX.Y.Z-rcN` to ensure the tag is correct.
        <details>
        # show the signed tag
        git show vX.Y.Z-rcN
        </details>
    - [ ] ⚠️ Push the `vX.Y.Z-rcN` tag to GitHub (`git push origin vX.Y.Z-rcN`; DO NOT USE `git push --tags` because it pushes all your local tags).
        <details>
        # show the signed tag
        git push origin vX.Y.Z-rcN
        </details>
  - [ ] Add artifacts to https://dist.ipfs.tech by making a PR against [ipfs/distributions](https://github.com/ipfs/distributions)
    - [ ] Clone the `ipfs/distributions` repo locally.
    - [ ] Create a new branch (`kubo-release-vX.Y.Z-rcn`) from `master`.
        <details>
        # checkout new branch
        git checkout -b kubo-release-vX.Y.Z-rcN
        </details>
    - [ ] Run `./dist.sh add-version kubo vX.Y.Z-rcN` to add the new version to the `versions` file ([instructions](https://github.com/ipfs/distributions#usage)).
      - `dist.sh` will print _WARNING: not marking pre-release kubo vX.Y.Z-rcNn as the current version._.
        <details>
        # add new kubo version to dist
        ./dist.sh add-version kubo vX.Y.Z-rcN
        git add dists/*/versions
        git commit -m "chore: add kubo vX.Y.Z-rcN"
        </details>
    - [ ] Push the `kubo-release-vX.Y.Z-rcn` branch to GitHub and create a PR from that branch ([example](https://github.com/ipfs/distributions/pull/760)).
        <details>
        # push the change
        git push origin kubo-release-vX.Y.Z-rcN

        # open a PR
        gh api /repos/ipfs/distributions/pulls \
          --method POST \
          -f title='chore: add kubo vX.Y.Z-rcN' \
          -f head='kubo-release-vX.Y.Z-rcN' \
          -f base='master'
        </details>
    - [ ] Ask for a review from the release reviewer.
    - [ ] Enable auto-merge for the PR.
      - PR build will build the artifacts and generate a diff in around 30 minutes
      - PR will be merged automatically once the diff is approved
      - `master` build will publish the artifacts to https://dist.ipfs.io in around 30 minutes
        <details>
        # get pull id
        id=$(gh api --method GET /repos/ipfs/distributions/pulls -f head='kubo-release-vX.Y.Z-rcN' --jq '.[0].node_id')

        # enable automerge
        gh api graphql -f pull="${id}" -f query='mutation($pull: ID!) { enablePullRequestAutoMerge(input: {pullRequestId: $pull, mergeMethod: SQUASH}) { pullRequest { autoMergeRequest { enabledAtenabledBy { login } } } } }'
        </details>
    - [ ] Ensure that the artifacts are available at https://dist.ipfs.io
        <details>
        # check if RC is available
        curl --retry 5 --no-progress-meter https://dist.ipfs.tech/kubo/versions | grep -q vX.Y.Z-rcN
        echo $?
        </details>
  - [ ] Publish the RC to [the NPM package](https://www.npmjs.com/package/go-ipfs?activeTab=versions) by running https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml (it happens automatically but it is safe to speed up the process and kick of a run manually)
      <details>
      # dispatch workflow
      gh api /repos/ipfs/npm-go-ipfs/actions/workflows/main.yml/dispatches \
        --method POST \
        -f ref='master'

      # get workflow run
      gh api /repos/ipfs/npm-go-ipfs/actions/workflows/main.yml/runs \
        --method GET \
        -f per_page='1' \
        --jq '.workflow_runs[0]'

      # get workflow job
      gh api /repos/ipfs/npm-go-ipfs/actions/runs/3470515021/jobs \
        --method GET \
        -f per_page='1' \
        --jq '.jobs[0]'

      # check logs for version
      gh api /repos/ipfs/npm-go-ipfs/actions/jobs/9499319520/logs \
        --method GET | grep -q 'X.Y.Z-rcN'
      echo $?
      </details>
  - [ ] Cut a pre-release on [GitHub](https://github.com/ipfs/kubo/releases) ([instructions](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository#creating-a-release), [example](https://github.com/ipfs/kubo/releases/tag/v0.16.0-rcN))
    - Use `vX.Y.Z-rcN` as the tag.
    - Link to the release issue in the description.
    - Link to the relevant [changelog](https://github.com/ipfs/kubo/blob/master/docs/changelogs/) in the description.
    - Check `This is a pre-release`.
      <details>
      # create a pre-release
      body='See the related issue: https://github.com/ipfs/kubo/issues/9319

      And the draft changelog: [docs/changelogs/v0.17.md](https://github.com/ipfs/kubo/blob/release-vX.Y.Z/docs/changelogs/v0.17.md)'
      gh api /repos/ipfs/kubo/releases \
        --method POST \
        -f tag_name='vX.Y.Z-rcN' \
        -f name='vX.Y.Z-rcN' \
        -f body="${body}" \
        -F draft=false \
        -F prerelease=true \
        -F generate_release_notes=false \
        -f make_latest=false
      </details>
  - [ ] Synchronize release artifacts by running [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow.
      <details>
      # dispatch workflow
      gh api /repos/ipfs/kubo/actions/workflows/sync-release-assets.yml/dispatches \
        --method POST \
        -f ref='master'
      </details>
  - [ ] Announce the RC
    - [ ] Create a new post on [IPFS Discourse](https://discuss.ipfs.tech). ([example](https://discuss.ipfs.tech/t/kubo-v0-16-0-rcN-release-candidate-is-out/15248))
      - Use `Kubo vX.Y.Z-rcn Release Candidate is out!` as the title.
      - Use `kubo` and `go-ipfs` as topics.
      - Repeat the title as a heading (`##`) in the description.
      - Link to the GitHub Release, binaries on IPNS, docker pull command and release notes in the description.
    - [ ] Pin the topic globally so that it stays at the top of the category.
    - [ ] If there is no more important banner currently set on Discourse (e.g. IPFS Camp announcement), make the topic into a banner.
    - [ ] Check if Discourse post was automatically copied to:
      - [ ] IPFS Discord #ipfs-chatter
      - [ ] FIL Slack #ipfs-chatter
      - [ ] Matrix https://matrix.to/#/#ipfs-chatter:ipfs.io
    - [ ] Mention [early testers](https://github.com/ipfs/go-ipfs/tree/master/docs/EARLY_TESTERS.md) in the comment under the release issue ([example](https://github.com/ipfs/kubo/issues/9319#issuecomment-1311002478)).
- [ ] **Stage 3 - Internal Testing**
  - [ ] Infrastructure Testing.
    - [ ] Update the issue against [bifrost-infra](https://github.com/protocol/bifrost-infra) ([example](https://github.com/protocol/bifrost-infra/issues/2109)).
      - [ ] Mention @protocol/bifrost-team in the issue to let them know the release is ready
      - [ ] [Optional] Reply under a message about the issue in the #bifrost channel on FIL Slack once the RC is out. Send the message to the channel.
    - [ ] Check [metrics](https://protocollabs.grafana.net/d/8zlhkKTZk/gateway-slis-precomputed?orgId=1) every day.
      - Compare the metrics trends week over week.
      - If there is an unexpected variation in the trend, message the #bifrost channel on FIL Slack and ask for help investigation the cause.
  - [ ] IPFS Application Testing.
    - [ ] [IPFS Desktop](https://github.com/ipfs-shipyard/ipfs-desktop)
      - [ ] Upgrade to the RC in [ipfs-desktop](https://github.com/ipfs-shipyard/ipfs-desktop)
      - [ ] Run `npm install` to update `package-lock.json`.
      - [ ] Push to a branch ([example](https://github.com/ipfs/ipfs-desktop/pull/1826/commits/b0a23db31ce942b46d95965ee6fe770fb24d6bde))
      - [ ] Open a draft PR to track through the final release ([example](https://github.com/ipfs/ipfs-desktop/pull/1826))
      - [ ] Ensure CI tests pass
        <details>
        # checkout new branch
        git checkout -b kubo-release-vX.Y.Z

        # replace the go-ipfs version in package.json
        sed -i 's/"go-ipfs": ".*"/"go-ipfs": "X.Y.Z-rcN"/' package.json

        # update package-lock.json
        npm install

        # commit the change
        git add package.json package-lock.json
        git commit -m "chore: bump kubo version to vX.Y.Z-rcN"

        # push the change
        git push origin kubo-release-vX.Y.Z

        # open a PR
        gh api /repos/ipfs/ipfs-desktop/pulls \
          --method POST \
          -f title='chore: bump kubo version to vX.Y.Z-rcN' \
          -f head='kubo-release-vX.Y.Z' \
          -f base='main' \
          -f title='chore: bump kubo version to vX.Y.Z' \
          -F draft=true
        </details>
    - [ ] [IPFS Companion](https://github.com/ipfs-shipyard/ipfs-companion)
      - [ ] Start kubo daemon of the version to release.
      - [ ] Start a fresh chromium or chrome instance using `chromium --user-data-dir=$(mktemp -d)` (macos `/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --user-data-dir=$(mktemp -d)`)
      - [ ] Start a fresh firefox instance using `firefox --profile $(mktemp -d)` (macos `/Applications/Firefox.app/Contents/MacOS/firefox --profile $(mktemp -d)`)
      - [ ] Install IPFS Companion from [vendor-specific store](https://github.com/ipfs/ipfs-companion/#readme).
      - [ ] Check that the comunication between Kubo daemon and IPFS companion is working properly checking if the number of connected peers changes.
        <details>
        curl --retry 5 --no-progress-meter --output kubo.tar.gz https://dist.ipfs.tech/kubo/vX.Y.Z-rcN/kubo_vX.Y.Z-rcN_darwin-arm64.tar.gz
        tar -xzvf kubo.tar.gz
        export IPFS_PATH=$(mktemp -d)
        ./kubo/ipfs init
        ./kubo/ipfs daemon &
        </details>
- [ ] **Stage 5 - Release** - _ONLY FOR FINAL RELEASE_
  - [ ] Prepare the `release` branch.
    - [ ] Bump the version in `version.go` in the `release-vX.Y.Z` branch to `X.Y.Z`.
        <details>
        # checkout new branch
        git checkout -b bump-version-vX.Y.Z

        # replace the version in version.go
        sed -i 's/const CurrentVersionNumber = ".*"/const CurrentVersionNumber = "X.Y.Z"/g' version.go

        # commit the change
        git add version.go
        git commit -m "chore: bump version to vX.Y.Z"

        # push the change
        git push origin bump-version-vX.Y.Z

        # open a PR
        gh api /repos/ipfs/kubo/pulls \
          --method POST \
          -f title='chore: bump version to vX.Y.Z' \
          -f head='bump-version-vX.Y.Z' \
          -f base='release-vX.Y.Z'
        </details>
    - [ ] Update the [docs/changelogs/vX.Y.md](docs/changelogs) with the new commits and contributors.
      - [ ] Run `./bin/mkreleaselog` twice to generate the changelog and copy the output.
      - The first run of the script might be poluted with `git clone` output.
      - [ ] Paste the output into the `### Changelog` section of the changelog file inside the `<details><summary></summary></details>` block.
      - [ ] Commit the changelog changes.
    - [ ] Push the `release-vX.Y.Z` branch to GitHub (`git push origin release-vX.Y.Z`)
    - [ ] Mark the PR created from `release-vX.Y.Z` as ready for review.
      - [ ] Ensure the PR is targetting `release` branch.
      - [ ] Ensure that CI is green.
      - [ ] Have release reviewer review the PR.
    - [ ] Merge the PR into `release` branch using the `Create a merge commit` (do **NOT** use `Squash and merge` nor `Rebase and merge` because we need to be able to sign the merge commit).
      - Do not delete the `release-vX.Y.Z` branch.
    - [ ] Checkout the `release` branch locally.
      - Remember to pull the latest changes.
      <details>
      git checkout release
      git pull
      </details>
    - [ ] Create a signed tag for the release.
      - [ ] This is a dangerous operation, as it is difficult to reverse due to Go modules and automated Docker image publishing. Remember to verify the commands you intend to run for items marked with ⚠️ with the release reviewer.
      - [ ] ⚠️ Tag HEAD `release` commit with `vX.Y.Z` (`git tag -s vX.Y.Z -m 'Release X.Y.Z'`)
      <details>
        # create a signed tag
        git tag -s vX.Y.Z -m 'Release X.Y.Z'
        </details>
      - [ ] Run `git show vX.Y.Z` to ensure the tag is correct.
        <details>
        # show the signed tag
        git show vX.Y.Z
        </details>
      - [ ] ⚠️ Push the `vX.Y.Z` tag to GitHub (`git push origin vX.Y.Z`; DO NOT USE `git push --tags` because it pushes all your local tags).
        <details>
        # show the signed tag
        git push origin vX.Y.Z
        </details>
  - [ ] Publish the release.
    - [ ] Wait for [Publish docker image](https://github.com/ipfs/kubo/actions/workflows/docker-image.yml) workflow run initiated by the tag push to finish.
    - [ ] Add artifacts to https://dist.ipfs.tech by making a PR against [ipfs/distributions](https://github.com/ipfs/distributions)
      - [ ] Clone the `ipfs/distributions` repo locally.
      - [ ] Create a new branch (`kubo-release-vX.Y.Z`) from `master`.
        <details>
        # create a new branch
        git checkout -b kubo-release-vX.Y.Z
        </details>
      - [ ] Run `./dist.sh add-version kubo vX.Y.Z` to add the new version to the `versions` file ([instructions](https://github.com/ipfs/distributions#usage)).
        <details>
        # add new kubo version to dist
        ./dist.sh add-version kubo vX.Y.Z
        git add dists/*/versions
        git commit -m "chore: add kubo vX.Y.Z"
        </details>
      - [ ] Push the `kubo-release-vX.Y.Z` branch to GitHub and create a PR from that branch ([example](https://github.com/ipfs/distributions/pull/768)).
        <details>
        # push the change
        git push origin kubo-release-vX.Y.Z

        # open a PR
        gh api /repos/ipfs/distributions/pulls \
          --method POST \
          -f title='chore: add kubo vX.Y.Z' \
          -f head='kubo-release-vX.Y.Z' \
          -f base='master'
        </details>
      - [ ] Ask for a review from the release reviewer.
      - [ ] Enable auto-merge for the PR.
        - PR build will build the artifacts and generate a diff in around 30 minutes
        - PR will be merged automatically once the diff is approved
        - `master` build will publish the artifacts to https://dist.ipfs.io in around 30 minutes
        <details>
        # get pull id
        id=$(gh api --method GET /repos/ipfs/distributions/pulls -f head='kubo-release-vX.Y.Z' --jq '.[0].node_id')

        # enable automerge
        gh api graphql -f pull="${id}" -f query='mutation($pull: ID!) { enablePullRequestAutoMerge(input: {pullRequestId: $pull, mergeMethod: SQUASH}) { pullRequest { autoMergeRequest { enabledBy { login } } } } }'
        </details>
      - [ ] Ensure that the artifacts are available at https://dist.ipfs.io
    - [ ] Publish the release to [the NPM package](https://www.npmjs.com/package/go-ipfs?activeTab=versions) by running https://github.com/ipfs/npm-go-ipfs/actions/workflows/main.yml (it happens automatically but it is safe to speed up the process and kick of a run manually)
  - [ ] Cut the release on [GitHub](https://github.com/ipfs/kubo/releases) ([instructions](https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository#creating-a-release), [example](https://github.com/ipfs/kubo/releases/tag/v0.16.0))
    - Use `vX.Y.Z` as the tag.
    - Copy the relevant [changelog](https://github.com/ipfs/kubo/blob/release/docs/changelogs/) into the release description.
    - Keep the release notes as trim as possible (e.g. remove top headers where possible, [example](https://github.com/ipfs/kubo/releases/tag/v0.15.0))
    <details>
    # create the release
    body="$(curl --retry 5 --no-progress-meter https://raw.githubusercontent.com/ipfs/kubo/release-vX.Y.Z/docs/changelogs/v0.17.md)"
    gh api /repos/ipfs/kubo/releases \
      --method POST \
      -f tag_name='vX.Y.Z' \
      -f name='vX.Y.Z' \
      -f body="${body}" \
      -F draft=false \
      -F prerelease=false \
      -F generate_release_notes=false \
      -f make_latest=true
    </details>
  - [ ] Synchronize release artifacts by running [sync-release-assets](https://github.com/ipfs/kubo/actions/workflows/sync-release-assets.yml) workflow.
      <details>
      # dispatch workflow
      gh api /repos/ipfs/kubo/actions/workflows/sync-release-assets.yml/dispatches \
        --method POST \
        -f ref='master'
      </details>
  - [ ] TODO: https://github.com/protocol/bifrost-infra/issues/2184#issuecomment-1315279257
  - [ ] Announce the release
    - [ ] Add a link to the release to this release issue as a comment.
    - [ ] Create a new post on [IPFS Discourse](https://discuss.ipfs.tech). ([example](https://discuss.ipfs.tech/t/kubo-v0-16-0-release-is-out/15286))
      - Use `Kubo vX.Y.Z Release is out!` as the title.
      - Use `kubo` and `go-ipfs` as topics.
      - Repeat the title as a heading (`##`) in the description.
      - Link to the GitHub Release, binaries on IPNS, docker pull command and release notes in the description.
    - [ ] Pin the topic globally so that it stays at the top of the category.
    - [ ] If there is no more important banner currently set on Discourse (e.g. IPFS Camp announcement), make the topic into a banner.
    - [ ] Check if Discourse post was automatically copied to:
      - [ ] IPFS Discord #ipfs-chatter
      - [ ] FIL Slack #ipfs-chatter
      - [ ] Matrix
  - [ ] Add a link from release notes to Discuss post (like we did here: https://github.com/ipfs/kubo/releases/tag/v0.15.0)
  - [ ] Update the draft PR created for [interop](https://github.com/ipfs/interop) to use the new release and mark it as ready for review.
  - [ ] Update the draft PR created for [IPFS Desktop](https://github.com/ipfs-shipyard/ipfs-desktop) to use the new release and mark it as ready for review.
  - [ ] Update docs
    - [ ] Run https://github.com/ipfs/ipfs-docs/actions/workflows/update-on-new-ipfs-tag.yml to generate a PR to the docs repo
    <details>
    # dispatch workflow
    gh api /repos/ipfs/ipfs-docs/actions/workflows/update-on-new-ipfs-tag.yml/dispatches \
      --method POST \
      -f ref='main'
    </details>
    - [ ] Merge the auto-created PR in https://github.com/ipfs/ipfs-docs/pulls ([example](https://github.com/ipfs/ipfs-docs/pull/1263))
  - [ ] Get the blog post created
    - [ ] Submit a request for blog post creation using [the form](https://airtable.com/shrNH8YWole1xc70I).
      - Title: Just released: Kubo X.Y.Z!
      - Link type: Release notes
      - URL: https://github.com/ipfs/kubo/releases/tag/vX.Y.Z
    - [ ] The post is live on https://blog.ipfs.io
    - [ ] Share the blog post
      - [ ] Twitter (request in Filecoin Slack channel #shared-pl-marketing-requests; [example](https://filecoinproject.slack.com/archives/C018EJ8LWH1/p1664903524843269?thread_ts=1664885305.374909&cid=C018EJ8LWH1))
      - [ ] [Reddit](https://reddit.com/r/ipfs)
- [ ] **Stage 6 - Post-Release**
  - [ ] Merge the `release` branch back into `master`, ignoring the changes to `version.go` (keep the `-dev` version from master).
  - [ ] Create an issue using this release issue template for the _next_ release.
  - [ ] Close this release issue.

## How to contribute?

Would you like to contribute to the IPFS project and don't know how? Well, there are a few places you can get started:

- Check the issues with the `help wanted` label in the [ipfs/kubo repo](https://github.com/ipfs/kubo/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22)
- Join the discussion at [discuss.ipfs.tech](https://discuss.ipfs.tech/) and help users finding their answers.
- See other options at https://docs.ipfs.tech/community/
