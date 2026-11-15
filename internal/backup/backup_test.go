package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geda-clipboard/internal/imageutil"
	"geda-clipboard/internal/statistics"
	"geda-clipboard/internal/store"
)

type fixture struct {
	dir   string
	stats *statistics.Store
	hist  *store.Store
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	hist, err := store.OpenAt(dir, 100)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := statistics.OpenAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		hist.Close()
		stats.Close()
	})
	return fixture{dir: dir, stats: stats, hist: hist}
}

func makePNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{shade, uint8(x * 20), uint8(y * 30), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func thumbnail(raw []byte) (string, int, int, error) {
	return imageutil.Thumbnail(raw, 480, 240)
}

// populate fills a source machine with text, a pinned entry, an image and a
// file reference carrying a sandbox bookmark.
func populate(t *testing.T, f fixture) {
	t.Helper()
	base := time.Now().Add(-time.Hour)
	const icon = "data:image/png;base64,SAFARIICON"
	captures := []store.Capture{
		{Kind: store.KindText, Text: "secret token", SourceApp: "Safari", SourceIconKey: "com.apple.Safari", SourceIcon: icon, At: base},
		{Kind: store.KindText, Text: "pinned snippet", SourceApp: "Notes", At: base.Add(time.Second)},
		{Kind: store.KindImage, Image: makePNG(t, 10), Thumb: "data:image/png;base64,T", ImageW: 8, ImageH: 6, SourceApp: "Preview", At: base.Add(2 * time.Second)},
		{Kind: store.KindFile, Files: []store.FileReference{{Path: "/Users/someone/report.pdf", Bookmark: "machine-grant"}}, SourceApp: "Finder", At: base.Add(3 * time.Second)},
	}
	for _, c := range captures {
		if _, _, err := f.hist.Add(c); err != nil {
			t.Fatal(err)
		}
	}
	for _, it := range f.hist.List("pinned snippet") {
		if _, err := f.hist.TogglePin(it.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.stats.Record(statistics.KindText, false); err != nil {
		t.Fatal(err)
	}
}

func export(t *testing.T, f fixture, opts Options) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "backup."+Extension)
	if _, err := WriteFile(out, opts, f.stats, f.hist, "0.0.0-test"); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRoundTripRestoresHistoryImagesAndStatistics(t *testing.T) {
	source := newFixture(t)
	populate(t, source)
	file := export(t, source, Options{IncludeHistory: true, IncludeImages: true})

	manifest, err := Inspect(file)
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.History || !manifest.Images || manifest.HistoryCount != 4 || manifest.ImageCount != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}

	target := newFixture(t)
	result, err := Import(file, target.dir, target.stats, target.hist, thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if result.History.Added != 4 || !result.Statistics {
		t.Fatalf("result = %+v", result)
	}

	items := target.hist.List("")
	if len(items) != 4 {
		t.Fatalf("restored %d entries, want 4", len(items))
	}
	if !items[0].Pinned || items[0].Text != "pinned snippet" {
		t.Fatalf("first entry = %+v, want the pinned snippet", items[0])
	}
	var imageID string
	for _, it := range items {
		if it.Kind == store.KindImage {
			imageID = it.ID
			if it.Thumb == "" || it.ImageW != 8 || it.ImageH != 6 {
				t.Fatalf("image entry not rebuilt: %+v", it)
			}
		}
		if it.Text == "secret token" && it.SourceIcon == "" {
			t.Fatal("source icon was not restored")
		}
	}
	raw, err := target.hist.ImageBytes(imageID)
	if err != nil || !bytes.Equal(raw, makePNG(t, 10)) {
		t.Fatalf("restored image differs: %v", err)
	}
	week, err := target.stats.Snapshot(statistics.RangeWeek)
	if err != nil || week.Totals.Total != 1 {
		t.Fatalf("restored statistics = %+v, %v", week.Totals, err)
	}
	if entries, _ := os.ReadDir(target.dir); hasStaging(entries) {
		t.Fatal("import left its staging directory behind")
	}
}

func hasStaging(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".import-") {
			return true
		}
	}
	return false
}

func TestImportTwiceChangesNothingTheSecondTime(t *testing.T) {
	source := newFixture(t)
	populate(t, source)
	file := export(t, source, Options{IncludeHistory: true, IncludeImages: true})

	target := newFixture(t)
	if _, err := Import(file, target.dir, target.stats, target.hist, thumbnail); err != nil {
		t.Fatal(err)
	}
	before := target.hist.List("")
	result, err := Import(file, target.dir, target.stats, target.hist, thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if result.History.Added != 0 || result.History.Merged != 4 {
		t.Fatalf("second import = %+v, want everything merged", result.History)
	}
	after := target.hist.List("")
	if len(after) != len(before) {
		t.Fatalf("entries %d -> %d after re-import", len(before), len(after))
	}
	for i := range after {
		if after[i].CopyCount != before[i].CopyCount || after[i].Pinned != before[i].Pinned {
			t.Fatalf("entry %d changed on re-import: %+v -> %+v", i, before[i], after[i])
		}
	}
	blobs, _ := os.ReadDir(filepath.Join(target.dir, "blobs"))
	if len(blobs) != 1 {
		t.Fatalf("blob directory holds %d files, want 1", len(blobs))
	}
	week, _ := target.stats.Snapshot(statistics.RangeWeek)
	if week.Totals.Total != 1 {
		t.Fatalf("statistics inflated to %d on re-import", week.Totals.Total)
	}
}

func TestExportWithoutImagesOmitsImageContent(t *testing.T) {
	source := newFixture(t)
	populate(t, source)
	file := export(t, source, Options{IncludeHistory: true, IncludeImages: false})

	zr, err := zip.OpenReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, imagesDir) {
			t.Fatalf("image entry %s exported with images off", f.Name)
		}
		if f.Name != historyName {
			continue
		}
		raw := readAll(t, f)
		var items []store.Item
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			if it.Kind == store.KindImage || it.Thumb != "" {
				t.Fatalf("image data exported with images off: %+v", it)
			}
		}
		if bytes.Contains(raw, []byte("machine-grant")) {
			t.Fatal("file bookmark was exported")
		}
	}
}

func TestStatisticsOnlyExportCarriesNoContent(t *testing.T) {
	source := newFixture(t)
	populate(t, source)
	file := export(t, source, Options{})
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != manifestName && f.Name != statisticsName {
			t.Fatalf("statistics-only backup contains %s", f.Name)
		}
		if bytes.Contains(readAll(t, f), []byte("secret token")) {
			t.Fatal("clipboard text leaked into a statistics-only backup")
		}
	}
}

func readAll(t *testing.T, f *zip.File) []byte {
	t.Helper()
	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	raw, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeArchive(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "crafted."+Extension)
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return out
}

func TestImportIgnoresEntriesOutsideTheAllowlist(t *testing.T) {
	manifest, _ := json.Marshal(Manifest{Format: Format, Version: Version, History: true})
	history, _ := json.Marshal([]store.Item{{
		Kind: store.KindImage, ImageFile: "../../escape.png", LastCopy: time.Now(),
	}, {
		Kind: store.KindText, Text: "kept", LastCopy: time.Now(),
	}})
	file := writeArchive(t, map[string][]byte{
		manifestName:              manifest,
		historyName:               history,
		"images/../../escape.png": makePNG(t, 1),
		"../outside.txt":          []byte("x"),
	})

	target := newFixture(t)
	result, err := Import(file, target.dir, target.stats, target.hist, thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if result.History.Added != 1 || result.History.Skipped != 1 {
		t.Fatalf("result = %+v, want the text kept and the image skipped", result.History)
	}
	for _, path := range []string{
		filepath.Join(filepath.Dir(target.dir), "escape.png"),
		filepath.Join(filepath.Dir(target.dir), "outside.txt"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("archive wrote outside the data directory: %s", path)
		}
	}
}

func TestImportRejectsNonBackups(t *testing.T) {
	target := newFixture(t)
	plain := filepath.Join(t.TempDir(), "notes.txt")
	os.WriteFile(plain, []byte("hello"), 0o600)
	if _, err := Import(plain, target.dir, target.stats, target.hist, thumbnail); !errors.Is(err, ErrNotBackup) {
		t.Fatalf("plain file: err = %v, want ErrNotBackup", err)
	}
	other := writeArchive(t, map[string][]byte{"readme.txt": []byte("x")})
	if _, err := Inspect(other); !errors.Is(err, ErrNotBackup) {
		t.Fatalf("foreign zip: err = %v, want ErrNotBackup", err)
	}
	newer, _ := json.Marshal(Manifest{Format: Format, Version: Version + 1})
	future := writeArchive(t, map[string][]byte{manifestName: newer})
	if _, err := Inspect(future); err == nil || errors.Is(err, ErrNotBackup) {
		t.Fatalf("newer backup: err = %v, want a version error", err)
	}
}

func TestDamagedStatisticsChangeNothing(t *testing.T) {
	manifest, _ := json.Marshal(Manifest{Format: Format, Version: Version, Statistics: true, History: true})
	history, _ := json.Marshal([]store.Item{{Kind: store.KindText, Text: "should not land", LastCopy: time.Now()}})
	file := writeArchive(t, map[string][]byte{
		manifestName:   manifest,
		statisticsName: []byte("{not json"),
		historyName:    history,
	})
	target := newFixture(t)
	if _, err := Import(file, target.dir, target.stats, target.hist, thumbnail); err == nil {
		t.Fatal("expected damaged statistics to fail the import")
	}
	if n := target.hist.Count(); n != 0 {
		t.Fatalf("history changed despite the failed import: %d entries", n)
	}
}

func TestImportMergesEntriesSharingOneImage(t *testing.T) {
	manifest, _ := json.Marshal(Manifest{Format: Format, Version: Version, History: true, Images: true})
	now := time.Now()
	history, _ := json.Marshal([]store.Item{
		{Kind: store.KindImage, ImageFile: "a.png", LastCopy: now, CopyCount: 2},
		{Kind: store.KindImage, ImageFile: "a.png", LastCopy: now.Add(-time.Minute), CopyCount: 5},
	})
	file := writeArchive(t, map[string][]byte{
		manifestName:        manifest,
		historyName:         history,
		imagesDir + "a.png": makePNG(t, 3),
	})
	target := newFixture(t)
	result, err := Import(file, target.dir, target.stats, target.hist, thumbnail)
	if err != nil {
		t.Fatal(err)
	}
	if result.History.Added != 1 || result.History.Merged != 1 {
		t.Fatalf("result = %+v, want one entry added and its duplicate merged", result.History)
	}
	items := target.hist.List("")
	if len(items) != 1 || items[0].CopyCount != 5 {
		t.Fatalf("restored = %+v, want one entry keeping the larger count", items)
	}
}
