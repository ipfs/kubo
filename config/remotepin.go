package config

const (
	PinningTag             = "Pinning"
	RemoteServicesTag      = "RemoteServices"
	RemoteServicesSelector = PinningTag + "." + RemoteServicesTag
)

type Pinning struct {
	RemoteServices map[string]RemotePinningService
}

type RemotePinningService struct {
	Api RemotePinningServiceApi
}

type RemotePinningServiceApi struct {
	Endpoint string
	Key      string
}
