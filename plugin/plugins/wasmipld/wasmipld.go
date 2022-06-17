package wasmipld

import (
	"io/ioutil"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"

	mc "github.com/multiformats/go-multicodec"

	"github.com/ipfs/go-ipfs/plugin"
	"github.com/mitchellh/mapstructure"

	wasmbind "github.com/aschmahmann/wasm-ipld/gobind"
)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&wasmipld{},
}

type wasmipld struct{}

var _ plugin.PluginIPLD = (*wasmipld)(nil)
var _ plugin.PluginIPLDADL = (*wasmipld)(nil)

func (*wasmipld) Name() string {
	return "ipld-wasmipld"
}

func (*wasmipld) Version() string {
	return "0.0.1"
}

type WasmIPLDConfig struct {
	Codecs []WasmIPLDCodecConfig
	ADLs   []WasmIPLDADLConfig
}

type WasmIPLDCodecConfig struct {
	Code     string
	Encode   bool
	Decode   bool
	WasmPath string
}

type WasmIPLDADLConfig struct {
	Name     string
	WasmPath string
}

func (*wasmipld) Init(env *plugin.Environment) error {
	config := env.Config
	if config == nil {
		return nil
	}

	var cfg WasmIPLDConfig
	// Note: This dependency is just for convenience and is in our go.sum anyway
	// It can go away if we upstream this into the main config instead of keeping it as a plugin
	if err := mapstructure.Decode(config, &cfg); err != nil {
		return err
	}

	registry = &wasmRegistry{}

	for _, c := range cfg.Codecs {
		wasm, err := ioutil.ReadFile(c.WasmPath)
		if err != nil {
			return err
		}

		var code mc.Code
		if err := code.Set(c.Code); err != nil {
			return err
		}

		codecRegistration := codecReg{
			code:   code,
			wasm:   wasm,
			encode: c.Encode,
			decode: c.Decode,
		}
		registry.Codecs = append(registry.Codecs, codecRegistration)
	}

	for _, a := range cfg.ADLs {
		wasm, err := ioutil.ReadFile(a.WasmPath)
		if err != nil {
			return err
		}

		adlRegistration := adlReg{
			name: a.Name,
			wasm: wasm,
		}

		registry.ADLs = append(registry.ADLs, adlRegistration)
	}

	return nil
}

const fuelPerOp = 10_000_000

var registry *wasmRegistry

type wasmRegistry struct {
	Codecs []codecReg
	ADLs   []adlReg
}

type codecReg struct {
	code   mc.Code
	wasm   []byte
	encode bool
	decode bool
}

type adlReg struct {
	name string
	wasm []byte
}

func (*wasmipld) Register(reg multicodec.Registry) error {
	reg.RegisterEncoder(wasmbind.WacMC, wasmbind.WacEncode)
	reg.RegisterDecoder(wasmbind.WacMC, wasmbind.WacDecode)

	for _, c := range registry.Codecs {
		codec, err := wasmbind.NewWasmCodec(c.wasm, wasmbind.WasmCodecOptions{}.WithFuelPerOp(fuelPerOp))
		if err != nil {
			return err
		}
		if c.encode {
			reg.RegisterEncoder(uint64(c.code), codec.Encode)
		}
		if c.decode {
			reg.RegisterDecoder(uint64(c.code), codec.Decode)
		}
	}
	return nil
}

func (b *wasmipld) RegisterADL(m map[string]ipld.NodeReifier) error {
	for _, a := range registry.ADLs {
		adl, err := wasmbind.NewWasmADL(a.wasm, wasmbind.WasmADLOptions{}.WithFuelPerOp(fuelPerOp))
		if err != nil {
			return err
		}
		m[a.name] = adl.Reify
	}
	return nil
}
