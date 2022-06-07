package wasmipld

import (
	wasmbind "github.com/aschmahmann/wasm-ipld/gobind"
	"io/ioutil"

	"github.com/ipfs/go-ipfs/plugin"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/multicodec"

	mc "github.com/multiformats/go-multicodec"
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

func (*wasmipld) Init(env *plugin.Environment) error {
	return nil
}

const fuelPerOp = 10_000_000

func init() {
	registry = &wasmRegistry{}

	wasm, err := ioutil.ReadFile("C:\\Users\\adin\\workspace\\rust\\wasm-ipld\\wasmlib\\target\\wasm32-unknown-unknown\\release\\bencode.wasm")
	if err != nil {
		panic(err)
	}
	bencodeCodec := codecReg{
		code:   mc.Bencode,
		wasm:   wasm,
		encode: true,
		decode: true,
	}

	wasm, err = ioutil.ReadFile("C:\\Users\\adin\\workspace\\rust\\wasm-ipld\\wasmlib\\target\\wasm32-unknown-unknown\\release\\bt_dirv1.wasm")
	if err != nil {
		panic(err)
	}
	registry.Codecs = append(registry.Codecs, bencodeCodec)

	const btFileADLName = "bittorrentv1-file"
	const btDirADLName = "bittorrentv1-directory"
	btDirV1Adl := adlReg{
		name: btDirADLName,
		wasm: wasm,
	}

	registry.ADLs = append(registry.ADLs, btDirV1Adl)
}

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
