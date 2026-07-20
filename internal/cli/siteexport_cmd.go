package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/EvilBit-Labs/unifi_extract/internal/extract"
	"github.com/EvilBit-Labs/unifi_extract/internal/siteexport"
	"github.com/spf13/cobra"
)

func newSitesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sites <backup-file>",
		Short: "List the sites contained in a full backup",
		Args:  cobra.ExactArgs(1),
	}
	f := addCommonFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		parsed, _, err := parseSites(f, args[0])
		if err != nil {
			return err
		}
		if len(parsed.Sites) == 0 {
			return fmt.Errorf("no sites found in %s (is this a full backup?)", args[0])
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Sites (%d):\n", len(parsed.Sites))
		for _, s := range parsed.Sites {
			hidden := ""
			if s.Hidden {
				hidden = "  [hidden]"
			}
			fmt.Fprintf(out, "  %s  %-20s %s%s\n", s.ID, s.Name, s.Desc, hidden)
		}
		return nil
	}
	return cmd
}

func newSiteExportCmd() *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:   "site-export <backup-file> --site <id-or-name>",
		Short: "Export one site from a full backup as an importable .unf",
		Args:  cobra.ExactArgs(1),
	}
	f := addCommonFlags(cmd)
	cmd.Flags().StringVar(&site, "site", "", "site id or name to export (required)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(site) == "" {
			return fmt.Errorf("--site is required (run 'sites %s' to list available sites)", args[0])
		}
		parsed, backup, err := parseSites(f, args[0])
		if err != nil {
			return err
		}
		target, err := parsed.FindSite(site)
		if err != nil {
			return err
		}
		version := readText(backup, "version", "backup/network/version", "network/version")
		if version == "" {
			version = "unknown"
		}
		extras := gatherSiteExtras(backup, target.Name)

		unf, err := parsed.BuildUnf(target, version, time.Now().UnixMilli(), extras)
		if err != nil {
			return err
		}
		out := f.out
		if out == "" {
			out = sanitizeFilename(target.Name) + ".unf"
		}
		if err := os.WriteFile(out, unf, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "exported site %q to %s (%d bytes)\n", target.Name, out, len(unf))
		return nil
	}
	return cmd
}

// parseSites opens a backup and decodes its MongoDB dump into sites.
func parseSites(f *commonFlags, file string) (*siteexport.Parsed, *extract.Backup, error) {
	backup, err := f.open(file)
	if err != nil {
		return nil, nil, err
	}
	dump, err := backup.MongoDump()
	if err != nil {
		return nil, nil, err
	}
	parsed, err := siteexport.Parse(dump)
	if err != nil {
		return nil, nil, err
	}
	return parsed, backup, nil
}

// gatherSiteExtras collects site-scoped files (floorplans, portal assets) from
// the source backup and remaps them to the "sites/<name>/..." export layout.
func gatherSiteExtras(backup *extract.Backup, siteName string) []siteexport.Extra {
	prefixes := []string{
		"backup/network/sites/" + siteName + "/",
		"sites/" + siteName + "/",
	}
	var extras []siteexport.Extra
	for _, prefix := range prefixes {
		for _, e := range backup.Entries {
			if rest, ok := strings.CutPrefix(e.Name, prefix); ok && rest != "" {
				extras = append(extras, siteexport.Extra{Name: "sites/" + siteName + "/" + rest, Data: e.Data})
			}
		}
		if len(extras) > 0 {
			break
		}
	}
	return extras
}

// sanitizeFilename makes a site name safe to use as a filename.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	cleaned := replacer.Replace(strings.TrimSpace(name))
	if cleaned == "" {
		return "site"
	}
	return cleaned
}
