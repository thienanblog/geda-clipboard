// Package backup writes and restores user-initiated Geda Clipboard backups.
//
// A backup is an ordinary ZIP archive with a .gedabackup extension. It is not
// encrypted: shipping our own cryptography would change the app's export
// compliance status, so the interface warns that a backup holding history is
// readable by anyone who has the file. Statistics are always included; history
// and its images are opt-in.
//
// Restoring merges rather than replaces, and every merge is idempotent, so a
// user can import the same file twice or restore into the machine that made it
// without inflating counts or duplicating entries.
package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"geda-clipboard/internal/statistics"
	"geda-clipboard/internal/store"
)

const (
	// Format identifies the archive; Version changes only for layouts older
	// readers cannot understand.
	Format  = "geda-clipboard-backup"
	Version = 1

	// Extension is the file extension offered by the save and open panels.
	Extension = "gedabackup"

	manifestName   = "manifest.json"
	statisticsName = "statistics.json"
	historyName    = "history.json"
	iconsName      = "icons.json"
	imagesDir      = "images/"

	// Size limits keep a damaged or hostile archive from exhausting memory or
	// disk. Each is far above what the app itself can produce.
	maxManifestBytes   = 64 << 10
	maxStatisticsBytes = 8 << 20
	maxHistoryBytes    = 1 << 30
	maxIconsBytes      = 32 << 20
	maxImageBytes      = 512 << 20
	maxImagePixels     = 64 << 20
)

// imageName accepts only flat blob names, so an archive entry can never
// address a path outside the staging directory.
var imageName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}\.png$`)

// ErrNotBackup reports a file that is not a Geda Clipboard backup at all.
var ErrNotBackup = errors.New("this file is not a Geda Clipboard backup")

// Options chooses what an export contains.
type Options struct {
	IncludeHistory bool `json:"includeHistory"`
	IncludeImages  bool `json:"includeImages"`
}

// Manifest describes an archive. It is read before anything is merged so the
// user can confirm what a file will restore.
type Manifest struct {
	Format       string    `json:"format"`
	Version      int       `json:"version"`
	CreatedAt    time.Time `json:"createdAt"`
	AppVersion   string    `json:"appVersion"`
	Statistics   bool      `json:"statistics"`
	History      bool      `json:"history"`
	Images       bool      `json:"images"`
	HistoryCount int       `json:"historyCount"`
	ImageCount   int       `json:"imageCount"`
}

// Result reports what an import changed.
type Result struct {
	Statistics bool              `json:"statistics"`
	History    store.MergeResult `json:"history"`
}

// Thumbnailer validates a PNG and returns the list thumbnail with the full
// image dimensions. The app supplies its own so restored entries match ones
// captured live.
type Thumbnailer func(png []byte) (thumb string, w, h int, err error)

// Write streams a backup to w. Image entries whose blob has disappeared since
// the snapshot are left out rather than failing the whole export.
func Write(w io.Writer, opts Options, stats *statistics.Store, history *store.Store, appVersion string) (Manifest, error) {
	if !opts.IncludeHistory {
		opts.IncludeImages = false
	}
	manifest := Manifest{
		Format:     Format,
		Version:    Version,
		CreatedAt:  time.Now().UTC(),
		AppVersion: appVersion,
		Statistics: stats != nil,
		History:    opts.IncludeHistory && history != nil,
		Images:     opts.IncludeImages && history != nil,
	}

	zw := zip.NewWriter(w)

	var statsRaw []byte
	if stats != nil {
		raw, err := stats.Export()
		if err != nil {
			return Manifest{}, fmt.Errorf("export statistics: %w", err)
		}
		statsRaw = raw
	}

	var items []store.Item
	var icons map[string]string
	if manifest.History {
		items, icons = history.ExportItems(opts.IncludeImages)
		kept := items[:0]
		for _, it := range items {
			if it.Kind != store.KindImage {
				kept = append(kept, it)
				continue
			}
			ok, err := copyImage(zw, history.BlobPath(it.ImageFile), it.ImageFile)
			if err != nil {
				return Manifest{}, err
			}
			if ok {
				kept = append(kept, it)
				manifest.ImageCount++
			}
		}
		items = kept
		manifest.HistoryCount = len(items)
	}

	// Metadata is written after the images so it lists only entries whose
	// payload actually made it into the archive.
	if err := writeJSON(zw, manifestName, manifest); err != nil {
		return Manifest{}, err
	}
	if statsRaw != nil {
		if err := writeRaw(zw, statisticsName, statsRaw); err != nil {
			return Manifest{}, err
		}
	}
	if manifest.History {
		if err := writeJSON(zw, historyName, items); err != nil {
			return Manifest{}, err
		}
		if err := writeJSON(zw, iconsName, icons); err != nil {
			return Manifest{}, err
		}
	}
	if err := zw.Close(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// WriteFile writes a backup to path, removing a partial file on failure.
func WriteFile(filePath string, opts Options, stats *statistics.Store, history *store.Store, appVersion string) (Manifest, error) {
	// The sandbox grants write access to the chosen file only, so the archive
	// is written in place rather than through a sibling temporary file.
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return Manifest{}, err
	}
	manifest, err := Write(f, opts, stats, history, appVersion)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(filePath)
		return Manifest{}, err
	}
	return manifest, nil
}

func copyImage(zw *zip.Writer, src, name string) (bool, error) {
	if !imageName.MatchString(name) {
		return false, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return false, nil
	}
	defer f.Close()
	// PNG data is already compressed; storing it avoids burning CPU for
	// nothing.
	w, err := zw.CreateHeader(&zip.FileHeader{Name: imagesDir + name, Method: zip.Store, Modified: time.Now()})
	if err != nil {
		return false, err
	}
	if _, err := io.Copy(w, f); err != nil {
		return false, fmt.Errorf("export image: %w", err)
	}
	return true, nil
}

func writeJSON(zw *zip.Writer, name string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeRaw(zw, name, raw)
}

func writeRaw(zw *zip.Writer, name string, raw []byte) error {
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}

// Inspect reads the manifest of the backup at filePath.
func Inspect(filePath string) (Manifest, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return Manifest{}, ErrNotBackup
	}
	defer zr.Close()
	_, manifest, err := indexArchive(&zr.Reader)
	return manifest, err
}

// Import merges the backup at filePath into stats and history. Images are
// staged in a temporary directory under stagingParent, which must be on the
// same volume as the history so staged files can be renamed into place.
// Nothing is merged until the whole archive has been read and validated.
func Import(filePath, stagingParent string, stats *statistics.Store, history *store.Store, thumbnail Thumbnailer) (Result, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return Result{}, ErrNotBackup
	}
	defer zr.Close()

	entries, manifest, err := indexArchive(&zr.Reader)
	if err != nil {
		return Result{}, err
	}

	var statsRaw []byte
	if manifest.Statistics && stats != nil {
		if f := entries[statisticsName]; f != nil {
			if statsRaw, err = readEntry(f, maxStatisticsBytes); err != nil {
				return Result{}, err
			}
			if err := statistics.Validate(statsRaw); err != nil {
				return Result{}, err
			}
		}
	}

	var items []store.Item
	icons := map[string]string{}
	if manifest.History && history != nil {
		f := entries[historyName]
		if f == nil {
			return Result{}, fmt.Errorf("backup is missing its history")
		}
		raw, err := readEntry(f, maxHistoryBytes)
		if err != nil {
			return Result{}, err
		}
		if err := json.Unmarshal(raw, &items); err != nil {
			return Result{}, fmt.Errorf("backup history is damaged")
		}
		if len(items) > store.MaxImportItems {
			return Result{}, fmt.Errorf("backup contains too many history entries")
		}
		if f := entries[iconsName]; f != nil {
			raw, err := readEntry(f, maxIconsBytes)
			if err != nil {
				return Result{}, err
			}
			// Icons are cosmetic; a damaged icon table only costs icons.
			_ = json.Unmarshal(raw, &icons)
		}
	}

	staging, err := os.MkdirTemp(stagingParent, ".import-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(staging)

	images := make(map[string]store.ImportImage)
	for _, it := range items {
		if it.Kind != store.KindImage || !imageName.MatchString(it.ImageFile) {
			continue
		}
		if _, done := images[it.ImageFile]; done {
			continue
		}
		f := entries[imagesDir+it.ImageFile]
		if f == nil {
			continue
		}
		staged, err := stageImage(f, staging, it.ImageFile, thumbnail)
		if err != nil {
			return Result{}, err
		}
		if staged != nil {
			images[it.ImageFile] = *staged
		}
	}

	var result Result
	if len(items) > 0 {
		merged, err := history.Merge(items, icons, images)
		if err != nil {
			return Result{}, fmt.Errorf("restore history: %w", err)
		}
		result.History = merged
	}
	if statsRaw != nil {
		if err := stats.Merge(statsRaw); err != nil {
			return result, fmt.Errorf("restore statistics: %w", err)
		}
		result.Statistics = true
	}
	return result, nil
}

// indexArchive maps allowlisted entry names to their files and reads the
// manifest. Unknown names are ignored, never extracted.
func indexArchive(zr *zip.Reader) (map[string]*zip.File, Manifest, error) {
	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		name := f.Name
		switch {
		case name == manifestName, name == statisticsName, name == historyName, name == iconsName:
		case path.Dir(name)+"/" == imagesDir && imageName.MatchString(path.Base(name)):
		default:
			continue
		}
		if _, dup := entries[name]; dup {
			return nil, Manifest{}, fmt.Errorf("backup is damaged")
		}
		entries[name] = f
	}

	f := entries[manifestName]
	if f == nil {
		return nil, Manifest{}, ErrNotBackup
	}
	raw, err := readEntry(f, maxManifestBytes)
	if err != nil {
		return nil, Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Format != Format {
		return nil, Manifest{}, ErrNotBackup
	}
	if manifest.Version > Version || manifest.Version < 1 {
		return nil, Manifest{}, fmt.Errorf("this backup was made by a newer version of Geda Clipboard")
	}
	return entries, manifest, nil
}

func readEntry(f *zip.File, limit int64) ([]byte, error) {
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("backup entry %s is too large", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("backup is damaged")
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, fmt.Errorf("backup is damaged")
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("backup entry %s is too large", f.Name)
	}
	return raw, nil
}

// stageImage extracts one image, validates it as a PNG and derives its
// thumbnail. An undecodable image is skipped rather than failing the restore.
func stageImage(f *zip.File, staging, name string, thumbnail Thumbnailer) (*store.ImportImage, error) {
	raw, err := readEntry(f, maxImageBytes)
	if err != nil {
		return nil, err
	}
	// Check the declared size before decoding: a small PNG can claim
	// dimensions whose pixel buffer would exhaust memory.
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
		return nil, nil
	}
	thumb, w, h, err := thumbnail(raw)
	if err != nil {
		return nil, nil
	}
	dest := filepath.Join(staging, name)
	if err := os.WriteFile(dest, raw, 0o600); err != nil {
		return nil, err
	}
	return &store.ImportImage{Path: dest, Thumb: thumb, W: w, H: h}, nil
}
