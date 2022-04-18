package wasmipld

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/bytecodealliance/wasmtime-go"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/multicodec"
	"io"
)

type wasmADLNode struct {
	ctx       linking.LinkContext
	lsys      *ipld.LinkSystem
	substrate ipld.Node

	k datamodel.Kind

	w *wasmtimeThings
	adlWasmPtr int32
}

var _ datamodel.LargeBytesNode = (*wasmADLNode)(nil)

// TODO: globals, caching, etc.
type wasmtimeThings struct {
	store *wasmtime.Store
	instance *wasmtime.Instance
	tfc      uint64
}

func newWasmADLNode(ctx linking.LinkContext, node datamodel.Node, lsys *linking.LinkSystem) (*wasmADLNode, error) {
	n := &wasmADLNode{
		ctx:       ctx,
		lsys:      lsys,
		substrate: node,
	}
	if err := n.initialize(); err != nil {
		return nil, err
	}
	return n, nil
}

func (w *wasmADLNode) initialize() error {
	// Almost all operations in wasmtime require a contextual `store`
	// argument to share, so create that first
	store := wasmtime.NewStore(wasmG.engine)

	if err := store.AddFuel(fuelPerOp); err != nil {
		return err
	}

	item := wasmtime.WrapFunc(store, func(caller *wasmtime.Caller, cidPtr int32, cidLen int32) int32 {
		memory := caller.GetExport("memory").Memory()
		buf := memory.UnsafeData(store)
		_, c, err := cid.CidFromBytes(buf[cidPtr: cidPtr + cidLen])
		if err != nil {
			return 0
		}
		alloc := caller.GetExport("myalloc").Func()
		blk, err := w.lsys.LoadRaw(w.ctx, cidlink.Link{Cid: c})

		const lenSize = 8 // add room for length pointer
		inputBlkPtrI, err := alloc.Call(caller, len(blk) + lenSize)
		inputBlkPtr, ok := inputBlkPtrI.(int32)
		if !ok {
			return 0
		}
		buf = memory.UnsafeData(store)
		binary.LittleEndian.PutUint64(buf[inputBlkPtr:inputBlkPtr + lenSize], uint64(len(blk)))
		copy(buf[inputBlkPtr+lenSize : inputBlkPtr + lenSize + int32(len(blk))], blk)
		return inputBlkPtr
	})

	item2 := wasmtime.WrapFunc(store, func(input int32) {
		// TODO: This function is a debug hook to be removed
		// fmt.Println(input)
	})

	// Next up we instantiate a module which is where we link in all our
	// imports. We've got one import so we pass that in here.
	instance, err := wasmtime.NewInstance(store, wasmG.module, []wasmtime.AsExtern{item2, item})
	if err != nil {
		return err
	}

	w.w = &wasmtimeThings{
		store:    store,
		instance: instance,
	}

	loadAdlFn := instance.GetExport(store, "load_adl").Func()
	memory := instance.GetExport(store,"memory").Memory()
	alloc := instance.GetExport(store,"myalloc").Func()

	// TODO: Figure out ADL type detection
	// could read functions, but that might not behave if there are multiple ADLs in a single store
	reader := instance.GetExport(store, "read_adl")
	if reader != nil {
		w.k = datamodel.Kind_Bytes
	}

	var inputBuf bytes.Buffer

	enc, err := multicodec.DefaultRegistry.LookupEncoder(WacMC)
	if err != nil {
		return err
	}

	if err := enc(w.substrate, &inputBuf); err != nil {
		return err
	}

	// // string for alloc
	inputSize := int32(inputBuf.Len())

	// //Allocate memory
	inputBlkPtrI, err := alloc.Call(store, inputSize)
	if err != nil {
		return err
	}
	inputBlkPtr, ok := inputBlkPtrI.(int32)
	if !ok {
		return fmt.Errorf("input block pointer not int32")
	}
	input := inputBuf.Bytes()

	// TODO: Dellocate input buffer

	buf := memory.UnsafeData(store)
	copy(buf[inputBlkPtr:], input)

	// Use load_adl func
	adlPtrI, err := loadAdlFn.Call(store, inputBlkPtr, inputSize)
	if err != nil {
		return err
	}
	adlPtr, ok := adlPtrI.(int32)
	if !ok {
		return fmt.Errorf("adl pointer not int32")
	}

	w.adlWasmPtr = adlPtr

	tfc, enabled := store.FuelConsumed()
	if !enabled {
		panic("how is fuel consumption not enabled?")
	}
	fc := tfc - w.w.tfc
	w.w.tfc = tfc
	fmt.Printf("Fuel consumed for ADL load: %d\n", fc)
	if err := store.AddFuel(fuelPerOp - fc); err != nil {
		return err
	}

	return nil
}

func (w *wasmADLNode) Kind() datamodel.Kind {
	return w.k
}

func (w *wasmADLNode) LookupByString(key string) (datamodel.Node, error) {
	panic("implement me")
}

func (w *wasmADLNode) LookupByNode(key datamodel.Node) (datamodel.Node, error) {
	panic("implement me")
}

func (w *wasmADLNode) LookupByIndex(idx int64) (datamodel.Node, error) {
	panic("implement me")
}

func (w *wasmADLNode) LookupBySegment(seg datamodel.PathSegment) (datamodel.Node, error) {
	panic("implement me")
}

func (w *wasmADLNode) MapIterator() datamodel.MapIterator {
	if w.k != datamodel.Kind_Map {
		return nil
	}
	panic("implement me")
}

func (w *wasmADLNode) ListIterator() datamodel.ListIterator {
	if w.k != datamodel.Kind_List {
		return nil
	}
	panic("implement me")
}

func (w *wasmADLNode) Length() int64 {
	panic("implement me")
}

func (w *wasmADLNode) IsAbsent() bool {
	// TODO: What should go here?
	return false
}

func (w *wasmADLNode) IsNull() bool {
	// TODO: Is this right?
	return w.k == datamodel.Kind_Null
}

func (w *wasmADLNode) AsBool() (bool, error) {
	if w.k != datamodel.Kind_Link {
		return false, ipld.ErrWrongKind{TypeName: "bool", MethodName: "AsBool", AppropriateKind: datamodel.KindSet{w.k}}
	}
	panic("implement me")
}

func (w *wasmADLNode) AsInt() (int64, error) {
	if w.k != datamodel.Kind_Link {
		return 0, ipld.ErrWrongKind{TypeName: "int", MethodName: "AsInt", AppropriateKind: datamodel.KindSet{w.k}}
	}
	panic("implement me")
}

func (w *wasmADLNode) AsFloat() (float64, error) {
	if w.k != datamodel.Kind_Link {
		return 0, ipld.ErrWrongKind{TypeName: "float", MethodName: "AsFloat", AppropriateKind: datamodel.KindSet{w.k}}
	}
	panic("implement me")
}

func (w *wasmADLNode) AsString() (string, error) {
	if w.k != datamodel.Kind_Link {
		return "", ipld.ErrWrongKind{TypeName: "string", MethodName: "AsString", AppropriateKind: datamodel.KindSet{w.k}}
	}
	panic("implement me")
}

func (w *wasmADLNode) AsBytes() ([]byte, error) {
	rdr, err := w.AsLargeBytes()
	if err != nil {
		return nil, err
	}
	return io.ReadAll(rdr)
}

func (w *wasmADLNode) AsLink() (datamodel.Link, error) {
	if w.k != datamodel.Kind_Link {
		return nil, ipld.ErrWrongKind{TypeName: "link", MethodName: "AsLink", AppropriateKind: datamodel.KindSet{w.k}}
	}
	panic("implement me")
}

func (w *wasmADLNode) Prototype() datamodel.NodePrototype {
	return nil
}

func (w *wasmADLNode) AsLargeBytes() (io.ReadSeeker, error) {
	if w.k != datamodel.Kind_Bytes {
		return nil, ipld.ErrWrongKind{TypeName: "bytes", MethodName: "AsLargeBytes", AppropriateKind: datamodel.KindSet{w.k}}
	}

	return &wasmADLRS{
		wt:     w.w,
		adlPtr: w.adlWasmPtr,
	}, nil
}

type wasmADLRS struct {
	wt *wasmtimeThings
	adlPtr int32
}

func (r *wasmADLRS) Read(p []byte) (n int, err error) {
	alloc := r.wt.instance.GetExport(r.wt.store,"myalloc").Func()
	readFn := r.wt.instance.GetExport(r.wt.store, "read_adl").Func()

	//Allocate memory
	bufferPtrI, err := alloc.Call(r.wt.store, len(p))
	if err != nil {
		return 0, err
	}
	bufferPtr, ok := bufferPtrI.(int32)
	if !ok {
		return 0, fmt.Errorf("buffer pointer not int32")
	}

	readI, err := readFn.Call(r.wt.store, r.adlPtr, bufferPtr, int32(len(p)))

	tfc, enabled := r.wt.store.FuelConsumed()
	if !enabled {
		panic("how is fuel consumption not enabled?")
	}
	fc := tfc - r.wt.tfc
	r.wt.tfc = tfc
	fmt.Printf("Fuel consumed for read: %d\n", fc)
	if err := r.wt.store.AddFuel(fuelPerOp - fc); err != nil {
		return 0, err
	}

	if err != nil {
		fmt.Println(err)
		return 0, err
	}
	read, ok := readI.(int32)
	if !ok {
		return 0, fmt.Errorf("read type not int32")
	}

	// TODO: Dellocate input buffer
	memory := r.wt.instance.GetExport(r.wt.store,"memory").Memory()
	buf := memory.UnsafeData(r.wt.store)
	numReturned := copy(p, buf[bufferPtr:bufferPtr + int32(read)])

	return numReturned, nil
}

func (r *wasmADLRS) Seek(offset int64, whence int) (int64, error) {
	seekFn := r.wt.instance.GetExport(r.wt.store, "seek_adl").Func()
	resI, err := seekFn.Call(r.wt.store, r.adlPtr, offset, int32(whence))

	tfc, enabled := r.wt.store.FuelConsumed()
	if !enabled {
		panic("how is fuel consumption not enabled?")
	}
	fc := tfc - r.wt.tfc
	r.wt.tfc = tfc
	fmt.Printf("Fuel consumed for seek: %d\n", fc)
	if err := r.wt.store.AddFuel(fuelPerOp - fc); err != nil {
		return 0, err
	}

	if err != nil {
		return 0, err
	}
	res, ok := resI.(int64)
	if !ok {
		return 0, fmt.Errorf("returned seek offset not a int64")
	}
	return res 	, nil
}

var _ io.ReadSeeker = (*wasmADLRS)(nil)
