package wasmipld

import (
	"bytes"
	"encoding/binary"
	"fmt"
	bencodeipld "github.com/aschmahmann/go-ipld-bittorrent/bencode"
	"github.com/ipld/go-ipld-prime/linking"
	"io"
	"io/ioutil"

	"github.com/bytecodealliance/wasmtime-go"

	"github.com/ipfs/go-ipfs/plugin"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
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

func (*wasmipld) Register(reg multicodec.Registry) error {
	reg.RegisterEncoder(uint64(mc.TorrentInfo), WacEncode)
	reg.RegisterDecoder(uint64(mc.TorrentInfo), WacDecode)

	reg.RegisterEncoder(uint64(mc.Bencode), bencodeipld.Encode)
	//reg.RegisterDecoder(uint64(mc.Bencode), bencodeipld.Decode)
	reg.RegisterDecoder(uint64(mc.Bencode), func(assembler datamodel.NodeAssembler, reader io.Reader) error {
		// Almost all operations in wasmtime require a contextual `store`
		// argument to share, so create that first
		store := wasmtime.NewStore(wasmtime.NewEngine())

		wasm, err := ioutil.ReadFile("C:\\Users\\adin\\workspace\\rust\\wasm-ipld\\target\\wasm32-unknown-unknown\\release\\wasm_ipld.wasm")
		if err != nil {
			return err
		}

		// Once we have our binary `wasm` we can compile that into a `*Module`
		// which represents compiled JIT code.
		module, err := wasmtime.NewModule(store.Engine, wasm)
		if err != nil {
			return err
		}

		item := wasmtime.WrapFunc(store, func(caller *wasmtime.Caller, cidPtr int32, cidLen int32) int32 {
			return 0
		})

		item2 := wasmtime.WrapFunc(store, func(input int32) {})

		// Next up we instantiate a module which is where we link in all our
		// imports. We've got one import so we pass that in here.
		instance, err := wasmtime.NewInstance(store, module, []wasmtime.AsExtern{item2, item})
		if err != nil {
			return err
		}

		fn := instance.GetExport(store, "decode").Func()
		memory := instance.GetExport(store,"memory").Memory()
		alloc := instance.GetExport(store,"myalloc").Func()

		block, err := ioutil.ReadAll(reader)
		if err != nil {
			return err
		}

		// // string for alloc
		size2 := int32(len(block))
		const lenSize = 8
		size2 += lenSize // add room for length pointer

		// //Allocate memory
		ptr2, err := alloc.Call(store, size2)
		pointer, _ := ptr2.(int32)

		buf := memory.UnsafeData(store)
		copy(buf[pointer+lenSize:], block)

		// Use decode func
		decodePtrI, err := fn.Call(store, pointer + lenSize, size2, pointer)
		if err != nil {
			return err
		}
		decodePtr, _ := decodePtrI.(int32)
		buf = memory.UnsafeData(store)

		dec, err := reg.LookupDecoder(uint64(mc.TorrentInfo))
		if err != nil {
			return err
		}

		outSize := (int32)(binary.LittleEndian.Uint64(buf[pointer:pointer+lenSize]))
		d := buf[decodePtr:decodePtr+outSize]
		fmt.Printf("%x\n", d)
		return dec(assembler, bytes.NewReader(d))
	})
	return nil
}

func (b *wasmipld) RegisterADL(m map[string]ipld.NodeReifier) error {
	const adlName = "bittorrentv1-file"
	//m[adlName] = bittorrentipld.ReifyBTFile
	m[adlName] = func(context linking.LinkContext, node datamodel.Node, system *linking.LinkSystem) (datamodel.Node, error) {
		return newWasmADLNode(context, node, system)
	}
	return nil
}
