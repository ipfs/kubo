# Security Policy

## Reporting a vulnerability

Email your report to **security@ipfs.io**. Please do not open a public issue.

Include whatever you have: the version or commit you tested, how to reproduce
the problem, and what an attacker gets out of it. A rough report is better than
no report, and we will ask if we need more.

A maintainer will confirm we received it and keep you posted while we work on a
fix. We are glad to credit you in the release notes, or to leave you out of them
if you would rather not be named.

If two weeks pass and no human has replied, assume the message never reached
one. Resend it, or escalate: the [OpenSSF finder guide](https://github.com/ossf/oss-vulnerability-guide/blob/main/finder-guide.md)
lays out the options, and [CERT/CC](https://kb.cert.org/vuls/report/) takes
reports when coordination with a project breaks down. We would rather you do
that than sit on a live bug.

If the problem is a design weakness that nobody can exploit today, or covers
something not yet released, it is fine to discuss it openly in an issue.

## Reporting abuse on a public gateway

Malware, phishing, or illegal material reachable through a public gateway is not
a bug in Kubo. Report it to whoever runs the gateway you used. For `ipfs.io` and
`dweb.link`, follow the [abuse policy](https://docs.ipfs.tech/concepts/public-utilities/#abuse-policy).

## Everything else

For normal bugs, [open an issue](https://github.com/ipfs/kubo/issues/new/choose).

This repository follows the [IPFS project security policy](https://github.com/ipfs/community/blob/master/SECURITY.md).
