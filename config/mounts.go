package config

// Mounts stores the (string) mount points.
type Mounts struct {
	IPFS           string
	IPNS           string
	MFS            string
	FuseAllowOther bool
}
