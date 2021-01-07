// +build linux,amd64

package main

import (
	"github.com/markbates/pkger"
	"github.com/markbates/pkger/pkging"
)

func openDistcc() (pkging.File, error) {
	return pkger.Open("/client/bin/distcc_amd64")
}
