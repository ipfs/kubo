//go:build mage
// +build mage

package main

import (
	//mage:import
	// _ "github.com/ipfs/kubo/internal/mage"

	//mage:import bifrost
	_ "github.com/ipfs/kubo/internal/mage/bifrost"

	//mage:import companion
	_ "github.com/ipfs/kubo/internal/mage/companion"

	//mage:import desktop
	_ "github.com/ipfs/kubo/internal/mage/desktop"

	//mage:import discourse
	_ "github.com/ipfs/kubo/internal/mage/discourse"

	//mage:import dist
	_ "github.com/ipfs/kubo/internal/mage/dist"

	//mage:import kubo
	_ "github.com/ipfs/kubo/internal/mage/kubo"

	//mage:import npm
	_ "github.com/ipfs/kubo/internal/mage/npm"
)
