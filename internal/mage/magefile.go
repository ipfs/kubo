//go:build mage
// +build mage

package main

import (
	//mage:import
	// _ "github.com/ipfs/kubo/internal/mage"

	//mage:import bifrost
	_ "github.com/ipfs/kubo/internal/mage/bifrost"

	//mage:import dist
	_ "github.com/ipfs/kubo/internal/mage/dist"

	//mage:import kubo
	_ "github.com/ipfs/kubo/internal/mage/kubo"
)
