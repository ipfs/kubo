package testutils

import "math/bits"

// VarintLen returns the number of bytes needed to encode v as a protobuf varint.
func VarintLen(v uint64) int {
	return int(9*uint32(bits.Len64(v))+64) / 64
}

// LinkSerializedSize calculates the serialized size of a single PBLink in a dag-pb block.
// This matches the calculation in boxo/ipld/unixfs/io/directory.go estimatedBlockSize().
//
// The protobuf wire format for a PBLink is:
//
//	PBNode.Links wrapper tag (1 byte)
//	+ varint length of inner message
//	+ Hash field: tag (1) + varint(cidLen) + cidLen
//	+ Name field: tag (1) + varint(nameLen) + nameLen
//	+ Tsize field: tag (1) + varint(tsize)
func LinkSerializedSize(nameLen, cidLen int, tsize uint64) int {
	// Inner link message size
	linkLen := 1 + VarintLen(uint64(cidLen)) + cidLen + // Hash field
		1 + VarintLen(uint64(nameLen)) + nameLen + // Name field
		1 + VarintLen(tsize) // Tsize field

	// Outer wrapper: tag (1 byte) + varint(linkLen) + linkLen
	return 1 + VarintLen(uint64(linkLen)) + linkLen
}

// EstimateFilesForBlockThreshold estimates how many files with given name/cid lengths
// will fit under the block size threshold.
// Returns the number of files that keeps the block size just under the threshold.
func EstimateFilesForBlockThreshold(threshold, nameLen, cidLen int, tsize uint64) int {
	linkSize := LinkSerializedSize(nameLen, cidLen, tsize)
	// Base overhead for empty directory node (Data field + minimal structure)
	// Empirically determined to be 4 bytes for dag-pb directories
	baseOverhead := 4
	return (threshold - baseOverhead) / linkSize
}
