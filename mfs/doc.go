// mfs stands for Mutable FileSystem
//
// package mfs implements an in memory model of a mutable ipfs filesystem.
//
// It consists of four main structs:
// 1) The Filesystem
//        The filesystem serves as a container and entry point for various mfs filesystems
// 2) Root
//        Root represents an individual filesystem mounted within the mfs system as a whole
// 3) Directories
// 4) Files
package mfs
