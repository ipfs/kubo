package gc

import (
	"context"

	dag "github.com/ipfs/go-ipfs/merkledag"

	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

const (
	colorNull trielement = iota
	color1
	color2
	color3
	colorMask = 0x3
)

const (
	// enum from enumerator
	// stricter enumerators should have higer value than those less strict
	enumFast   trielement = 0
	enumStrict trielement = 1 << 2
	enumMask   trielement = enumStrict
)

// this const is used to enable runtime check for things that should never happen
// will cause panic if they happen
const pedantic = true

// try to keep trielement as small as possible
// 8bit color is overkill, 3 bits would be enough, but additional functionality
// will probably increase it future
// if ever it blows over 64bits total size change the colmap in triset to use
// pointers onto this structure
type trielement uint8

func (t trielement) getColor() trielement {
	return t & colorMask
}

func (t trielement) getEnum() trielement {
	return t & enumMask
}

type triset struct {
	// colors are per triset allowing fast color swap after sweep,
	// the update operation in map of structs is about 3x as fast
	// as insert and requres 0 allocations (keysize allocation in case of insert)
	white, gray, black trielement

	freshColor trielement

	// grays is used as stack to enumerate elements that are still gray
	grays []*cid.Cid

	// if item doesn't exist in the colmap it is treated as white
	colmap map[string]trielement
}

func newTriset() *triset {
	tr := &triset{
		white: color1,
		gray:  color2,
		black: color3,

		grays:  make([]*cid.Cid, 0, 1<<10),
		colmap: make(map[string]trielement),
	}

	tr.freshColor = tr.white
	return tr
}

// InsertFresh inserts fresh item into a set
// it marks it with freshColor if it is currently white
func (tr *triset) InsertFresh(c *cid.Cid) {
	e := tr.colmap[c.KeyString()]
	cl := e.getColor()

	// conditions to change the element:
	// 1. does not exist in set
	// 2. is white and fresh color is different
	if cl == colorNull || (cl == tr.white && tr.freshColor != tr.white) {
		tr.colmap[c.KeyString()] = trielement(tr.freshColor)
	}
}

// InsertWhite inserts white item into a set if it doesn't exist
func (tr *triset) InsertWhite(c *cid.Cid) {
	_, ok := tr.colmap[c.KeyString()]
	if !ok {
		tr.colmap[c.KeyString()] = trielement(tr.white)
	}
}

// InsertGray inserts new item into set as gray or turns white item into gray
// strict arguemnt is used to signify the the garbage collector that this
// DAG must be enumerated fully, any non aviable objects must stop the progress
// and error out
func (tr *triset) InsertGray(c *cid.Cid, strict bool) {
	newEnum := enumFast
	if strict {
		newEnum = enumStrict
	}

	e := tr.colmap[c.KeyString()]
	cl := e.getColor()
	// conditions are:
	// 1. empty
	// 2. white
	// 3. insufficient strictness
	if cl == colorNull || cl == tr.white || (e.getEnum() < newEnum) {
		tr.colmap[c.KeyString()] = trielement(tr.gray | newEnum)
		if cl != tr.gray {
			tr.grays = append(tr.grays, c)
		}
	}
}

func (tr *triset) blacken(c *cid.Cid, strict trielement) {
	tr.colmap[c.KeyString()] = trielement(tr.black | strict)
}

// EnumerateStep performs one Links lookup in search for elements to gray out
// it returns error is the getLinks function errors
// if the gray set is empty after this step it returns (true, nil)
func (tr *triset) EnumerateStep(ctx context.Context, getLinks dag.GetLinks, getLinksStrict dag.GetLinks) (bool, error) {
	var c *cid.Cid
	var e trielement
	for next := true; next; next = e.getColor() != tr.gray {
		if len(tr.grays) == 0 {
			return true, nil
		}
		// get element from top of queue
		c = tr.grays[len(tr.grays)-1]
		e = tr.colmap[c.KeyString()]
		tr.grays = tr.grays[:len(tr.grays)-1]
	}

	strict := e.getEnum() == enumStrict

	// select getLinks method
	gL := getLinks
	if strict {
		gL = getLinksStrict
	}

	links, err := gL(ctx, c)
	if err != nil {
		return false, err
	}

	tr.blacken(c, e.getEnum())
	for _, l := range links {
		tr.InsertGray(l.Cid, strict)
	}

	return len(tr.grays) == 0, nil
}
