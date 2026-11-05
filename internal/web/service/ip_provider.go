// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// Client IPs are labelled from local MMDB files; lookups never leave the
// server. A MaxMind GeoLite2 file the operator places in the database folder
// or the bin folder (GeoLite2-ASN.mmdb, GeoLite2-City.mmdb) wins. Otherwise
// the panel keeps the free DB-IP Lite files (CC BY 4.0) current itself — no
// cron job: it downloads the monthly file as a plain public HTTPS GET, with no
// account, key or client data, into the database folder (/etc/x-ui, kept by
// install/update and a Docker volume), and refreshes it once it is older than
// ipDBMaxAge. A copy in the bin folder (older releases) is still used.
type ipDatabase struct {
	sync.Mutex
	files       []string // preferred first; the last one is downloadable
	urlFmt      string   // DB-IP URL, %s = yyyy-mm
	path        string
	reader      *maxminddb.Reader
	modTime     time.Time
	lastTry     time.Time
	downloading bool
}

const (
	ipDBMaxAge     = 35 * 24 * time.Hour
	ipDBRetryAfter = 6 * time.Hour
)

// ipDBDownloads turns the DB-IP download off; test binaries never download.
var ipDBDownloads = !testing.Testing()

var (
	ipASNDB = &ipDatabase{
		files:  []string{"GeoLite2-ASN.mmdb", "dbip-asn-lite.mmdb"},
		urlFmt: "https://download.db-ip.com/free/dbip-asn-lite-%s.mmdb.gz",
	}
	ipCityDB = &ipDatabase{
		files:  []string{"GeoLite2-City.mmdb", "dbip-city-lite.mmdb"},
		urlFmt: "https://download.db-ip.com/free/dbip-city-lite-%s.mmdb.gz",
	}
)

// IPProvider is the network and location an IP belongs to. Every field is
// optional: whatever the local databases don't cover stays empty.
type IPProvider struct {
	ASN          uint              `json:"asn,omitempty"`
	Org          string            `json:"org,omitempty"`
	Country      string            `json:"country,omitempty"` // ISO code
	CountryNames map[string]string `json:"countryNames,omitempty"`
	City         string            `json:"city,omitempty"`
	// Source names the databases used ("maxmind", "dbip" or both, comma
	// separated) so the panel can credit them.
	Source string `json:"source,omitempty"`
}

// geoNameLangs are the country-name languages passed to the panel.
var geoNameLangs = []string{"en", "ru", "fa", "zh-CN", "es", "pt-BR", "de", "fr", "ja"}

// LookupIPProvider labels ip from the local databases.
func LookupIPProvider(ip string) IPProvider {
	var out IPProvider
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return out
	}
	addr = addr.Unmap()
	var sources []string
	addSource := func(s string) {
		for _, have := range sources {
			if have == s {
				return
			}
		}
		sources = append(sources, s)
	}
	if r, src := ipASNDB.open(); r != nil {
		var rec struct {
			ASN uint   `maxminddb:"autonomous_system_number"`
			Org string `maxminddb:"autonomous_system_organization"`
		}
		if r.Lookup(addr).Decode(&rec) == nil && (rec.ASN != 0 || rec.Org != "") {
			out.ASN, out.Org = rec.ASN, rec.Org
			addSource(src)
		}
	}
	if r, src := ipCityDB.open(); r != nil {
		var rec struct {
			City struct {
				Names map[string]string `maxminddb:"names"`
			} `maxminddb:"city"`
			Country struct {
				ISO   string            `maxminddb:"iso_code"`
				Names map[string]string `maxminddb:"names"`
			} `maxminddb:"country"`
		}
		if r.Lookup(addr).Decode(&rec) == nil && rec.Country.ISO != "" {
			out.Country = rec.Country.ISO
			out.City = rec.City.Names["en"]
			for _, l := range geoNameLangs {
				if n := rec.Country.Names[l]; n != "" {
					if out.CountryNames == nil {
						out.CountryNames = map[string]string{}
					}
					out.CountryNames[l] = n
				}
			}
			addSource(src)
		}
	}
	out.Source = strings.Join(sources, ",")
	return out
}

func ipDBSource(name string) string {
	if strings.HasPrefix(name, "GeoLite2") {
		return "maxmind"
	}
	return "dbip"
}

// open returns a reader for the first database file present, reopening it
// when the file is replaced, or nil when none is there yet. It starts a
// background download when the DB-IP file is missing or stale.
func (d *ipDatabase) open() (*maxminddb.Reader, string) {
	d.Lock()
	defer d.Unlock()
	dirs := ipDBDirs()
	dbip := d.files[len(d.files)-1]
	if newest := newestFile(dirs, dbip); newest.IsZero() || time.Since(newest) > ipDBMaxAge {
		d.refreshLocked(dirs[0], dbip)
	}
	for _, name := range d.files {
		path, st := freshestPath(dirs, name)
		if path == "" {
			continue
		}
		if d.reader == nil || d.path != path || !st.ModTime().Equal(d.modTime) {
			r, err := maxminddb.Open(path)
			if err != nil {
				logger.Warning("ip database: open", path, "failed:", err)
				continue
			}
			if d.reader != nil {
				_ = d.reader.Close()
			}
			d.reader, d.path, d.modTime = r, path, st.ModTime()
		}
		return d.reader, ipDBSource(name)
	}
	if d.reader != nil {
		_ = d.reader.Close()
		d.reader, d.path = nil, ""
	}
	return nil, ""
}

// ipDBDirs lists where database files are looked for, download target first.
func ipDBDirs() []string {
	dirs := []string{config.GetDBFolderPath()}
	if bin := config.GetBinFolderPath(); bin != dirs[0] {
		dirs = append(dirs, bin)
	}
	return dirs
}

// freshestPath returns the newest copy of name across dirs.
func freshestPath(dirs []string, name string) (string, os.FileInfo) {
	var best string
	var bestSt os.FileInfo
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && st.Size() > 0 && (bestSt == nil || st.ModTime().After(bestSt.ModTime())) {
			best, bestSt = p, st
		}
	}
	return best, bestSt
}

func newestFile(dirs []string, name string) time.Time {
	if _, st := freshestPath(dirs, name); st != nil {
		return st.ModTime()
	}
	return time.Time{}
}

func (d *ipDatabase) refreshLocked(dir, name string) {
	if !ipDBDownloads || d.urlFmt == "" || d.downloading || time.Since(d.lastTry) < ipDBRetryAfter {
		return
	}
	d.downloading, d.lastTry = true, time.Now()
	go func() {
		defer func() {
			d.Lock()
			d.downloading = false
			d.Unlock()
		}()
		dst := filepath.Join(dir, name)
		now := time.Now().UTC()
		// The new month's file is published during the first days of it.
		for _, month := range []time.Time{now, now.AddDate(0, -1, 0)} {
			url := fmt.Sprintf(d.urlFmt, month.Format("2006-01"))
			err := fetchGzipTo(url, dst)
			if err == nil {
				logger.Info("ip database: updated", name, "from", url)
				// An older copy elsewhere (bin/ on releases before this
				// one) is now superseded; drop it instead of keeping 130 MB.
				for _, other := range ipDBDirs() {
					if other != dir {
						_ = os.Remove(filepath.Join(other, name))
					}
				}
				return
			}
			logger.Warning("ip database:", url, err)
		}
	}()
}

// fetchGzipTo downloads a gzipped MMDB, checks that it opens, and only then
// replaces dst, so a bad download never removes a working file.
func fetchGzipTo(url, dst string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, io.LimitReader(gz, 1<<30)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	r, err := maxminddb.Open(tmpName)
	if err != nil {
		return fmt.Errorf("invalid database: %w", err)
	}
	_ = r.Close()
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}
