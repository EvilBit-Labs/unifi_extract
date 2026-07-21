// Package siteexport extracts a single site from a full UniFi backup and
// repackages it as an importable .unf site export.
//
// A UniFi MongoDB dump is a command stream: a document {__cmd:"select",
// collection:"X"} selects a collection, and the documents that follow are its
// rows until the next select. Per-site collections carry a site_id string that
// references the owning site's _id. Exporting a site means emitting that site's
// row plus, for each per-site collection, only the rows whose site_id matches.
package siteexport

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/EvilBit-Labs/unifi_extract/internal/crypto"
	"github.com/EvilBit-Labs/unifi_extract/internal/mongodump"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// siteCollections is the set of per-site collections a site export includes,
// in the same order the reference implementation emits them.
var siteCollections = []string{
	"account", "acl_rule", "alert_setting", "apgroup", "device", "dhcpoption",
	"doh_servers", "dpiapp", "dpigroup", "dynamicdns", "event", "firewallgroup",
	"firewallrule", "firewall_policy", "firewall_zone", "floorplan_file", "guest",
	"heatmap", "heatmappoint", "hotspot2conf", "hotspotop", "hotspotpackage", "map",
	"networkconf", "payment", "portconf", "portforward", "portalfile", "radiusprofile",
	"qos_rule", "rogue", "rogueknown", "routing", "scheduletask", "simple_app_block",
	"site_feature_migration", "setting", "switch_stacking", "tag", "traffic_route",
	"traffic_rule", "user", "usergroup", "virtualdevice", "voucher", "wall",
	"wireguard_user", "wlanconf", "wlangroup",
}

const selectCmd = "select"

// Site describes a site discovered in a full backup.
type Site struct {
	ID     string
	Name   string
	Desc   string
	Hidden bool
	docRaw []byte // the raw BSON bytes of the site's own document
}

type row struct {
	siteID string
	raw    []byte
}

// Parsed is the decoded command stream of a full backup's MongoDB dump.
type Parsed struct {
	Sites       []Site
	collections map[string][]row
}

// Parse decodes a decompressed MongoDB dump (the raw concatenated-BSON stream)
// into its sites and per-collection rows.
func Parse(dump []byte) (*Parsed, error) {
	p := &Parsed{collections: make(map[string][]row)}
	current := ""
	err := mongodump.ForEach(dump, func(_ int, doc bson.Raw) error {
		if cmd, ok := doc.Lookup("__cmd").StringValueOK(); ok && cmd == selectCmd {
			current, _ = doc.Lookup("collection").StringValueOK()
			return nil
		}
		if current == "" {
			return nil
		}
		raw := append([]byte(nil), doc...)
		siteID := valueToString(doc.Lookup("site_id"))
		p.collections[current] = append(p.collections[current], row{siteID: siteID, raw: raw})
		if current == "site" {
			p.Sites = append(p.Sites, siteFromDoc(doc, raw))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func siteFromDoc(doc bson.Raw, raw []byte) Site {
	id := valueToString(doc.Lookup("_id"))
	name, _ := doc.Lookup("name").StringValueOK()
	desc, hasDesc := doc.Lookup("desc").StringValueOK()
	if name == "" {
		name = "unknown"
	}
	if !hasDesc || desc == "" {
		desc = name
	}
	hidden, _ := doc.Lookup("attr_hidden").BooleanOK()
	return Site{ID: id, Name: name, Desc: desc, Hidden: hidden, docRaw: raw}
}

// FindSite returns the site whose ID or name matches selector, or an error
// naming the available sites when there is no unambiguous match.
func (p *Parsed) FindSite(selector string) (Site, error) {
	var matches []Site
	for _, s := range p.Sites {
		if s.ID == selector || s.Name == selector {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return Site{}, fmt.Errorf("no site matches %q; available: %s", selector, p.siteNames())
	default:
		return Site{}, fmt.Errorf("selector %q is ambiguous; select by id instead: %s", selector, p.siteNames())
	}
}

func (p *Parsed) siteNames() string {
	names := make([]string, len(p.Sites))
	for i, s := range p.Sites {
		names[i] = fmt.Sprintf("%s (%s)", s.Name, s.ID)
	}
	return fmt.Sprintf("%v", names)
}

// Extra is an extra file (floorplan, portal asset) copied into the export.
type Extra struct {
	Name string
	Data []byte
}

// BuildUnf assembles and encrypts an importable .unf for the given site.
// version is the source controller version; timestampMillis stamps the export;
// extras are site-scoped files already remapped to "sites/<name>/..." paths.
func (p *Parsed) BuildUnf(site Site, version string, timestampMillis int64, extras []Extra) ([]byte, error) {
	dbGz, err := p.buildSiteDump(site)
	if err != nil {
		return nil, err
	}

	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	files := []Extra{
		{Name: "db.gz", Data: dbGz},
		{Name: "version", Data: []byte(version)},
		{Name: "timestamp", Data: []byte(strconv.FormatInt(timestampMillis, 10))},
	}
	files = append(files, extras...)
	for _, f := range files {
		if unsafeArchiveName(f.Name) {
			return nil, fmt.Errorf("refusing to write unsafe archive entry name %q into export", f.Name)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			return nil, fmt.Errorf("zip create %s: %w", f.Name, err)
		}
		if _, err := w.Write(f.Data); err != nil {
			return nil, fmt.Errorf("zip write %s: %w", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("zip close: %w", err)
	}

	return crypto.EncryptCBC(zbuf.Bytes(), crypto.MustHex(crypto.UnfKeyHex), crypto.MustHex(crypto.UnfIVHex))
}

// buildSiteDump rebuilds the concatenated-BSON dump for one site and gzips it.
func (p *Parsed) buildSiteDump(site Site) ([]byte, error) {
	var stream bytes.Buffer
	if err := writeSelect(&stream, "site"); err != nil {
		return nil, err
	}
	stream.Write(site.docRaw)

	for _, name := range siteCollections {
		if err := writeSelect(&stream, name); err != nil {
			return nil, err
		}
		for _, r := range p.collections[name] {
			if r.siteID == site.ID {
				stream.Write(r.raw)
			}
		}
	}

	var gz bytes.Buffer
	gw := gzip.NewWriter(&gz)
	if _, err := gw.Write(stream.Bytes()); err != nil {
		return nil, fmt.Errorf("gzip write site dump: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("gzip close site dump: %w", err)
	}
	return gz.Bytes(), nil
}

func writeSelect(buf *bytes.Buffer, collection string) error {
	b, err := bson.Marshal(bson.D{{Key: "__cmd", Value: selectCmd}, {Key: "collection", Value: collection}})
	if err != nil {
		return fmt.Errorf("marshal select %s: %w", collection, err)
	}
	buf.Write(b)
	return nil
}

// unsafeArchiveName reports whether a zip entry name would escape the archive
// root when later extracted (path traversal or an absolute path). Extra files
// are copied from the source backup with backup-derived names, so a crafted
// backup must not be able to bake a "../"-bearing entry into an exported .unf.
func unsafeArchiveName(name string) bool {
	if name == "" {
		return true
	}
	// Normalize Windows separators so "..\.." is caught alongside "../..".
	normalized := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(normalized, "/") { // absolute path
		return true
	}
	clean := path.Clean(normalized)
	return clean == ".." || strings.HasPrefix(clean, "../")
}

// valueToString mirrors JavaScript String(v) for the id fields we key on:
// an ObjectID becomes its hex string, a string stays itself. Anything else
// (or an absent field) yields "".
func valueToString(v bson.RawValue) string {
	if oid, ok := v.ObjectIDOK(); ok {
		return oid.Hex()
	}
	if s, ok := v.StringValueOK(); ok {
		return s
	}
	return ""
}
