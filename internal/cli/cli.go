// Package cli implements the unifi-extract command-line interface using
// Cobra commands wrapped by Charm's Fang for a polished experience.
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/EvilBit-Labs/unifi_extract/internal/extract"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

const longDescription = `unifi-extract decrypts and explores UniFi backup files (.unf and .unifi).

Everything runs locally: the tool never touches the network, so backup
contents never leave your machine.

Formats:
  .unf    site export or classic controller autobackup (AES-128-CBC -> zip)
  .unifi  UniFi OS / UCore console full backup (AES-256-CBC -> gzip -> tar)`

// Execute builds the command tree and runs it through Fang.
func Execute(ctx context.Context, version string) error {
	return fang.Execute(ctx, newRootCmd(), fang.WithVersion(version))
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "unifi-extract",
		Short: "Decrypt and explore UniFi backup files (.unf, .unifi)",
		Long:  longDescription,
		Example: `  unifi-extract info backup.unf
  unifi-extract decrypt backup.unf -o backup.zip
  unifi-extract extract console.unifi -o ./console
  unifi-extract mongo backup.unf > docs.ndjson
  unifi-extract sites console.unifi
  unifi-extract site-export console.unifi --site Default -o default.unf`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newInfoCmd(), newDecryptCmd(), newExtractCmd(), newMongoCmd(),
		newSitesCmd(), newSiteExportCmd(),
	)
	return root
}

// commonFlags holds the -o / --type flags shared by every subcommand.
type commonFlags struct {
	out  string
	kind string
}

func addCommonFlags(cmd *cobra.Command) *commonFlags {
	f := &commonFlags{}
	cmd.Flags().StringVarP(&f.out, "out", "o", "", "output path (file or directory, per command)")
	cmd.Flags().StringVar(&f.kind, "type", "", `force format: "unf" or "unifi" (default: by extension)`)
	return f
}

// open decrypts the backup at file, honoring the --type override.
func (f *commonFlags) open(file string) (*extract.Backup, error) {
	kind, err := parseKind(f.kind)
	if err != nil {
		return nil, err
	}
	return extract.Open(file, kind)
}

func parseKind(s string) (extract.Kind, error) {
	switch strings.ToLower(s) {
	case "":
		return "", nil
	case "unf":
		return extract.KindUnf, nil
	case "unifi":
		return extract.KindUnifi, nil
	default:
		return "", fmt.Errorf("invalid --type %q (want \"unf\" or \"unifi\")", s)
	}
}
