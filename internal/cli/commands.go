package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/EvilBit-Labs/unifi_extract/internal/extract"
	"github.com/EvilBit-Labs/unifi_extract/internal/mongodump"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func newInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <backup-file>",
		Short: "Summarize a backup (type, version, timestamp, entries, doc count)",
		Args:  cobra.ExactArgs(1),
	}
	f := addCommonFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		b, err := f.open(args[0])
		if err != nil {
			return err
		}
		return writeInfo(cmd.OutOrStdout(), args[0], b)
	}
	return cmd
}

func writeInfo(w io.Writer, file string, b *extract.Backup) error {
	fmt.Fprintf(w, "File:    %s\n", file)
	fmt.Fprintf(w, "Type:    .%s  (%s)\n", b.Kind, containerDesc(b.Kind))
	if v := readText(b, "version", "backup/network/version", "network/version"); v != "" {
		fmt.Fprintf(w, "Version: %s\n", v)
	}
	if ts := readText(b, "timestamp", "backup/network/timestamp", "network/timestamp"); ts != "" {
		fmt.Fprintf(w, "Time:    %s\n", ts)
	}

	fmt.Fprintf(w, "\nEntries (%d):\n", len(b.Entries))
	sizes := make(map[string]int, len(b.Entries))
	names := make([]string, len(b.Entries))
	for i, e := range b.Entries {
		names[i] = e.Name
		sizes[e.Name] = len(e.Data)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  %10d  %s\n", sizes[n], n)
	}

	if raw, err := b.MongoDump(); err == nil {
		if n, err := mongodump.Count(raw); err == nil {
			fmt.Fprintf(w, "\nMongoDB documents: %d\n", n)
		}
	}
	return nil
}

func newDecryptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decrypt <backup-file>",
		Short: "Write the raw decrypted container (.zip for .unf, .tar for .unifi)",
		Args:  cobra.ExactArgs(1),
	}
	f := addCommonFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		b, err := f.open(args[0])
		if err != nil {
			return err
		}
		out := f.out
		if out == "" {
			out = defaultName(args[0], b.ContainerExt)
		}
		if err := os.WriteFile(out, b.Container, 0o600); err != nil {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s (%d bytes)\n", out, len(b.Container))
		return nil
	}
	return cmd
}

func newExtractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract <backup-file>",
		Short: "Unpack every file from the backup into a directory",
		Args:  cobra.ExactArgs(1),
	}
	f := addCommonFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		b, err := f.open(args[0])
		if err != nil {
			return err
		}
		dir := f.out
		if dir == "" {
			dir = defaultName(args[0], "_extracted")
		}
		for _, e := range b.Entries {
			dest, err := safeJoin(dir, e.Name)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, e.Data, 0o600); err != nil {
				return err
			}
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "extracted %d entries to %s/\n", len(b.Entries), dir)
		return nil
	}
	return cmd
}

func newMongoCmd() *cobra.Command {
	var pretty bool
	cmd := &cobra.Command{
		Use:   "mongo <backup-file>",
		Short: "Decode the MongoDB dump to newline-delimited JSON (NDJSON)",
		Args:  cobra.ExactArgs(1),
	}
	f := addCommonFlags(cmd)
	cmd.Flags().BoolVar(&pretty, "pretty", false, "indent each document (multi-line)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		b, err := f.open(args[0])
		if err != nil {
			return err
		}
		raw, err := b.MongoDump()
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		err = mongodump.ForEach(raw, func(_ int, doc bson.Raw) error {
			line, err := mongodump.ToExtJSON(doc)
			if err != nil {
				return err
			}
			if pretty {
				if line, err = indentJSON(line); err != nil {
					return err
				}
			}
			buf.Write(line)
			buf.WriteByte('\n')
			return nil
		})
		if err != nil {
			return err
		}
		return writeOut(f.out, cmd.OutOrStdout(), buf.Bytes(), cmd.ErrOrStderr())
	}
	return cmd
}

func containerDesc(k extract.Kind) string {
	if k == extract.KindUnifi {
		return "AES-256-CBC -> gzip -> tar"
	}
	return "AES-128-CBC -> zip"
}

// readText returns the trimmed text of the first matching entry, or "".
func readText(b *extract.Backup, candidates ...string) string {
	if e := b.Find(candidates...); e != nil {
		return string(bytes.TrimSpace(e.Data))
	}
	return ""
}
