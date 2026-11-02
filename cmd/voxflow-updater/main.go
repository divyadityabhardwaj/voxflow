// voxflow-updater finishes an update after the UI process has exited.
package main

import (
	"fmt"
	"os"
	"voxflow/internal/update"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: voxflow-updater <installation plan>")
		os.Exit(2)
	}
	if err := update.RunInstaller(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
