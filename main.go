// Command unifi-extract decrypts and explores UniFi backup files locally.
package main

import (
	"context"
	"os"

	"github.com/EvilBit-Labs/unifi_extract/internal/cli"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := cli.Execute(context.Background(), version); err != nil {
		os.Exit(1)
	}
}
