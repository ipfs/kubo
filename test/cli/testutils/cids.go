package testutils

// Well-known CIDs that a fresh repo produces. The empty directory has the same
// multihash in both CID versions (it has no children). The welcome-docs
// directory does not: the unixfs-v1-2025 profile stores its files as raw
// leaves, so the CIDv1 directory has different links (and a different
// multihash) than the CIDv0 one. Tests that only care about the content, not
// the version, can accept either form.
const (
	// Empty UnixFS directory.
	CIDEmptyDirV0 = "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"
	CIDEmptyDirV1 = "bafybeiczsscdsbs7ffqz55asqdf3smv6klcw3gofszvwlyarci47bgf354"

	// Init welcome-docs directory.
	CIDWelcomeDocsV0 = "QmQPeNsJPyVWPFDVHb77w8G42Fvo15z4bG2X8D2GhfbSXc"
	CIDWelcomeDocsV1 = "bafybeiacej34nzxqmsrc3gacsygou27mqo5m44vvcfepgnprojzsdw7rvi"

	// Defaults match what `ipfs init` writes today: the unixfs-v1-2025 import
	// profile, so welcome docs come out as CIDv1.
	CIDWelcomeDocs = CIDWelcomeDocsV1
	// An arbitrary well-known CID used only as input to routing lookups, where
	// the version is irrelevant.
	CIDEmptyDir = CIDEmptyDirV0
)
