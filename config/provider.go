package config

const (
	DefaultProviderWorkerCount = 64
)

type Provider struct {
	Strategy    string // Which keys to announce
	WorkerCount uint   // Number of concurrent provides allowed, 0 means unlimited
}
