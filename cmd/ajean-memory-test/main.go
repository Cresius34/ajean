package main

import (
	"os"

	_ "embed"
)

//go:embed resource_windows_amd64.syso
var _ amd64Res

//go:embed resource_windows_arm64.syso
var _ arm64Res

func main() {
	os.Exit(0)
}
