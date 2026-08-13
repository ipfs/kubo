# Contributing to Kubo

**For development setup, building, and testing, see the [Developer Guide](docs/developer-guide.md).**

[AGENTS.md](AGENTS.md) holds the build and test workflow plus the hard rules a change must not break. It is written for AI coding agents, and it is just as useful if you are a human working with an LLM assistant: point your tool at it, and skim it yourself before opening a PR.

## What Kubo optimizes for

Kubo has run in production since 2015, and a decade of software, scripts, and services depends on it behaving predictably. Backward compatibility comes before new features and before internal elegance. The RPC API on `/api/v0/`, the HTTP Gateway, the CIDs produced by default, and the wire protocols are contracts with everyone who already built on Kubo; breaking them quietly breaks people who trusted the project. The specific rules a change must not cross are in [AGENTS.md](AGENTS.md#stability-what-you-must-not-break).

Kubo exists so people can run their own infrastructure and address their own data without asking anyone's permission. Most of its users are individuals and small operators self-hosting a node, not large platforms. That shapes the defaults we pick:

- **Design for the person running a node on their own hardware.** Give them control, defaults they can see and change, and no dependency they cannot route around.
- **Every hardcoded endpoint or reliance on shared infrastructure is configurable and can be turned off.** No operator should be locked into a URL, bootstrap peer, router, or certificate authority the maintainers picked. `AutoConf` is the working example.
- **Defaults serve the self-hoster.** Anything that phones home or narrows an operator's options is opt-in; we do not opt people in for them.
- **No new lock-in.** Be wary of changes that only make sense for one large deployment while making the software worse for someone running a single node. Kubo is a public good, and it should stay useful to the people who cannot fund their own fork. "Who does this default serve?" is a fair question to ask of any change.

## Guidelines

IPFS as a project, including Kubo and all of its modules, follows the [standard IPFS Community contributing guidelines](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md).

We also adhere to the [Go IPFS Community contributing guidelines](https://github.com/ipfs/community/blob/master/CONTRIBUTING_GO.md) which provide additional information on how to collaborate and contribute to the Go implementation of IPFS.

We appreciate your time and attention for going over these. Please open an issue on ipfs/community if you have any questions.

Thank you.
