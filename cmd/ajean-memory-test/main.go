// ajean-memory-test is the portable proof build. The regular cmd/ajean entry
// and its platform resources retain their official identity and defaults.
package main

import "github.com/nathaninline/ajean/internal/ajean"

//go:generate goversioninfo -64 -icon=../ajean/icon.ico -o resource_windows_amd64.syso versioninfo.json
//go:generate goversioninfo -64 -arm -icon=../ajean/icon.ico -o resource_windows_arm64.syso versioninfo.json

func main() {
	ajean.BuildFlavor = "embedding-test"
	ajean.Main()
}
