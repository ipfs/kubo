# Publishing go-ipfs as a snap

> Snap is the default package manager for ubuntu since the release of 20.04. This doc captures what we know about building go-ipfs as a snap packge and publishing it to the snapstore.

The go-ipfs snap is defined in [snap/snapcraft.yaml](https://github.com/ipfs/go-ipfs/blob/master/snap/snapcraft.yaml). For more detail on our snapcraft.yaml see: https://github.com/ipfs-shipyard/ipfs-snap

- go-ipfs is published as `ipfs` to the snapcraft store, see: https://snapcraft.io/ipfs
- ipfs-desktop is published as `ipfs-desktop`, from CI, here: https://github.com/ipfs-shipyard/ipfs-desktop/blob/master/.github/workflows/snapcraft.yml

For go-ipfs we deliberately lean on the Canonical lauchpad.net build environment so as it simplifies creating builds for more architectures, which has been requested by user numerous times.

Linux user can install go-ipfs with:

```
$ snap install ipfs
```

Apps installed via Snapcraft are auto-updating by default. Snapcraft uses 'Release Channels' to let the user pick their stability level, with channels for `stable`, `candidate`, `beta` and `edge`. Snap will install the lasest release from the `stable` channel by default. A user that wants to test out the bleeding edge can opt in by passing the `--edge` flag

```
$ snap install --edge ipfs
```

<img width="1072" alt="Dashboard for managaing go-ipfs snap release channels for go-ipfs" src="https://user-images.githubusercontent.com/58871/95761096-bcb34580-0ca3-11eb-8ce7-2496b2456335.png">

## Known issues

- `ipfs mount` fails as fusermount is not included in the snap, and cannot work from a snap as it is not able to create non-root mounts, see: https://github.com/elopio/ipfs-snap/issues/6

```console
ubuntu@primary:~$ ipfs mount
2020-07-10T09:54:17.458+0100	ERROR	node	node/mount_unix.go:91	error mounting: fusermount: exec: "fusermount": executable file not found in $PATH
2020-07-10T09:54:17.463+0100	ERROR	node	node/mount_unix.go:95	error mounting: fusermount: exec: "fusermount": executable file not found in $PATH
```

## Developing

We let launchpad.net build our snap for us, but if you need to edit the snapcraft.yml you can test it locally

### Requirements

 You need `snapcraft` installed locally

```console
# ubuntu or similar
$ snap install snapcraft --classic

# macos
$ brew install snapcraft
```

### Build and test

**Build** out a snap package for go-ipfs by running the following from this project

```console
$ snapcraft
```

**Test** the built snap package by installing it on a system that has `snapd`

```
$ snap install ipfs_<snap details here>.snap
# then kick the tires
$ ubuntu@primary:~$ ipfs daemon
Initializing daemon...
go-ipfs version: 0.7.0-dev
```

You can test it out on mac too. By installing and using `snapcraft`, it'll pull in `multipass` which is a quick way to run an ubuntu vm, and it has a notion of a primary container, which gets nice things like automounting your home dir in the vm, so you can:

```console
# install your .snap in a multipass vm
$ multipass shell
ubuntu@primary:~$ cd ~/Home/path/to/snap/on/host/filesystem
ubuntu@primary:~$ snap install ipfs_<snap details>.snap --devmode --dangerous
ubuntu@primary:~$ ipfs daemon
Initializing daemon...
go-ipfs version: 0.7.0-dev
```

### Building in Docker

[ipfs-shipyard/ipfs-snap](https://github.com/ipfs-shipyard/ipfs-snap) includes a Dockerfile that creates an image that can build go-ipfs from source and package it as a snap. It starts with `snapcore/snapcraft:stable` and adds in `go` and just enough tools to allow snapcraft to build go-ipfs. It is published to dockerhub as `ipfs/ipfs-snap-builder`.

```console
$ docker run -v $(pwd):/my-snap ipfs/ipfs-snap-builder:latest sh -c "apt update && cd /my-snap && snapcraft --debug"
```

## Publishing the Snap

The following snap release channels are published automatically:

| Git branch | Snap channel |
|------------|--------------|
| `release`  | `stable`
| `master`   | `edge`


### Edge via snapcraft.io

The snapcraft store watches the default branch of the go-ipfs repo, and updates the snap for the `edge` channel. This service is configured automatically by snapcraft. It's neat, but it doesn't allow us to watch any branch other than the default.

<img width="1072" alt="Screenshot 2020-10-12 at 15 56 07" src="https://user-images.githubusercontent.com/58871/95761075-b755fb00-0ca3-11eb-99d4-95e5f42cb48a.png">


### Stable via launchpad.net

The `stable` channel is published automatically via launchpad.net. There is a mirror of the go-ipfs repo at https://launchpad.net/go-ipfs that is sync'd with the github repo every few hours (at canonical's leisure).

A snap build configuration called `ipfs-stable` is set up to watch the `release` branch on go-ipfs and publish it to the `stable` snap channel.

The key configuration points are:

```yaml
# What flavour VM to build on.
Series: infer from snapcraft.yml

Source:
    Git:
        # the launchpad mirror of go-ipfs
        Git repository: ~ipfs/go-ipfs/+git/go-ipfs
        Git branch: refs/heads/release

Automatically build when branch changes: true
    Source snap channels for automatic builds:
        # tell snapcraft what version of snapcraft to use when building.
        # NOTE: At time of writing we use the default `core18` platform for the 
        # go-ipfs snap. If you specify others here, a build error occurs, which 
        # I think is mainly due to a launchpad ux bug here.
        core: ""
        core18: stable
        core20: ""
        snapcraft: stable


Automatically upload to store:
    Registered store package name: ipfs
    Store channels:
        Risk: 
            Stable: true

# What architectures to build for. this selection is chosen to match the auto 
# configured build provided by snapcraft for the edge channel, for neatness, so 
# that all architectures that currently have builds in snap continue to get 
# updates, even though some of them would be tough for use to test on.
Processors:
    amd64: true
    # raspi 4
    arm64: true
    # older raspi
    armhf: true
    # sure ok i guess.
    i386: true
    # hmmm... PowerPC!?
    ppc64el: true
    # wat. IBM system Z mainframes!?
    s390x: true
```

![Screenshot_2020-10-12 Edit ipfs-stable Snap packages “IPFS Maintainers” team](https://user-images.githubusercontent.com/58871/95762510-b4f4a080-0ca5-11eb-8148-d208f891d202.png)

### Future work - Publish RCs to the `candidate` channel

If we wish to publish release candidates to the snap store, we can do that by creating a new snap build config

1. Find the `release-vX.X` branch in the lauchpad.net mirror of the go-ipfs repo.
    - e.g. https://code.launchpad.net/~ipfs/go-ipfs/+git/go-ipfs/+ref/release-v0.7.0
2. Click "Create snap package"
3. Fill out the form using the same values as listed above for the stable channel, but:
    - Set `Name` to `ipfs-candidate` _(this just needs to be a unique name to identify this config)_
    - For `Risk` select only `Candidate` _(so the snap is published to the `Candidate` channel.)_

You can trigger a build manually to kick things off. Subsequent changes to that branch will be published as a snap automatically when when the mirror next syncs with github (every 6-12hrs)


## Who can edit this?

The `ipfs` snapcraft.io listing can be edited by

- @elopio _TBC, the original submitter, need to check about getting ownership transferred._
- @lidel
- @olizilla

You need a Canonical developer account, then ask an existing owner to add you. Accsess is managed here https://dashboard.snapcraft.io/snaps/ipfs/collaboration/


The launchpad.net config is managed by [**IPFS Maintainers**](https://launchpad.net/~ipfs) team, and you can request to join that team with your Canonical developer acccount. The list of maintainers is here: https://launchpad.net/~ipfs/+members

At the time of writing the launchpad maintainers are:

- @lidel
- @olizilla


## References

- Walkthrough of publishing a snap package via snapcraft and launchpad: https://www.youtube.com/watch?v=X_U-pcvBFrU
- For more details on the go-ipfs snapcraft.yaml see: https://github.com/ipfs-shipyard/ipfs-snap
- publishing to multiple channels via build.snapcraft.io: https://forum.snapcraft.io/t/maintaining-and-publishing-multiple-to-multiple-channels-via-build-snapcraft-io/12455
- How node.js manages snaps: https://github.com/ipfs/go-ipfs/issues/7679#issuecomment-695914986
