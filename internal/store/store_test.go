package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newStore(t *testing.T, maxItems int) *Store {
	t.Helper()
	s, err := OpenAt(t.TempDir(), maxItems)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func addText(t *testing.T, s *Store, text string, at time.Time) (*Item, bool) {
	t.Helper()
	it, isNew, err := s.Add(Capture{Kind: KindText, Text: text, At: at, SourceApp: "TestApp"})
	if err != nil {
		t.Fatalf("Add(%q): %v", text, err)
	}
	return it, isNew
}

func TestAddNewText(t *testing.T) {
	s := newStore(t, 10)
	base := time.Now()

	it, isNew := addText(t, s, "hello", base)
	if !isNew {
		t.Error("first add should report a new entry")
	}
	if it.CopyCount != 1 {
		t.Errorf("CopyCount = %d, want 1", it.CopyCount)
	}
	if it.Kind != KindText || it.Text != "hello" {
		t.Errorf("unexpected item: %+v", it)
	}
	if s.Count() != 1 {
		t.Errorf("Count = %d, want 1", s.Count())
	}
}

// Re-copying identical content must bump the existing entry rather than
// duplicating it -- this is what drives the "Number of copies" figure.
func TestAddDuplicateBumpsCount(t *testing.T) {
	s := newStore(t, 10)
	base := time.Now()

	addText(t, s, "hello", base)
	addText(t, s, "world", base.Add(time.Second))

	it, isNew := addText(t, s, "hello", base.Add(2*time.Second))
	if isNew {
		t.Error("duplicate add should not report a new entry")
	}
	if it.CopyCount != 2 {
		t.Errorf("CopyCount = %d, want 2", it.CopyCount)
	}
	if s.Count() != 2 {
		t.Errorf("Count = %d, want 2 (no duplicate row)", s.Count())
	}

	// The bumped entry must now be first.
	list := s.List("")
	if list[0].Text != "hello" {
		t.Errorf("front entry = %q, want %q", list[0].Text, "hello")
	}
	// FirstCopy must be preserved, LastCopy advanced.
	if !list[0].FirstCopy.Equal(base) {
		t.Errorf("FirstCopy = %v, want %v", list[0].FirstCopy, base)
	}
	if !list[0].LastCopy.Equal(base.Add(2 * time.Second)) {
		t.Errorf("LastCopy = %v, want %v", list[0].LastCopy, base.Add(2*time.Second))
	}
}

func TestEvictionRespectsMax(t *testing.T) {
	s := newStore(t, 3)
	base := time.Now()

	for i, text := range []string{"a", "b", "c", "d", "e"} {
		addText(t, s, text, base.Add(time.Duration(i)*time.Second))
	}

	if s.Count() != 3 {
		t.Fatalf("Count = %d, want 3", s.Count())
	}
	list := s.List("")
	got := []string{list[0].Text, list[1].Text, list[2].Text}
	want := []string{"e", "d", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("list[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPinnedSurviveEvictionAndSortFirst(t *testing.T) {
	s := newStore(t, 2)
	base := time.Now()

	first, _ := addText(t, s, "keep-me", base)
	if _, err := s.TogglePin(first.ID); err != nil {
		t.Fatalf("TogglePin: %v", err)
	}

	for i, text := range []string{"b", "c", "d", "e"} {
		addText(t, s, text, base.Add(time.Duration(i+1)*time.Second))
	}

	// 2 unpinned + 1 pinned.
	if s.Count() != 3 {
		t.Fatalf("Count = %d, want 3", s.Count())
	}

	list := s.List("")
	if !list[0].Pinned || list[0].Text != "keep-me" {
		t.Errorf("pinned entry should sort first, got %+v", list[0])
	}

	// Unpinning should let it be evicted on the next add.
	if _, err := s.TogglePin(first.ID); err != nil {
		t.Fatalf("TogglePin off: %v", err)
	}
	if s.Count() != 2 {
		t.Errorf("Count after unpin = %d, want 2", s.Count())
	}
	if _, ok := s.Get(first.ID); ok {
		t.Error("unpinned stale entry should have been evicted")
	}
}

func TestSearch(t *testing.T) {
	s := newStore(t, 10)
	base := time.Now()

	addText(t, s, "Hello World", base)
	addText(t, s, "goodbye", base.Add(time.Second))

	if got := s.List("hello"); len(got) != 1 || got[0].Text != "Hello World" {
		t.Errorf("case-insensitive search failed: %+v", got)
	}
	if got := s.List("TESTAPP"); len(got) != 2 {
		t.Errorf("search by source app returned %d, want 2", len(got))
	}
	if got := s.List("nope"); len(got) != 0 {
		t.Errorf("unmatched search returned %d, want 0", len(got))
	}
}

// "ima" should find images, but a single letter must not: matching every image
// on "a", "e", "g", "i" or "m" would swamp the list on the first keystroke.
func TestSearchMatchesImagesOnlyOnAMeaningfulPrefix(t *testing.T) {
	s := newStore(t, 10)

	if _, _, err := s.Add(Capture{
		Kind:  KindImage,
		Image: pngBytes(t, 2, 2, color.RGBA{0, 0, 255, 255}),
		At:    time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	addText(t, s, "unrelated text", time.Now())

	for _, needle := range []string{"im", "ima", "image"} {
		if got := s.List(needle); len(got) != 1 || got[0].Kind != KindImage {
			t.Errorf("List(%q) = %d entries, want the one image", needle, len(got))
		}
	}
	for _, needle := range []string{"a", "e", "g", "i", "m"} {
		for _, it := range s.List(needle) {
			if it.Kind == KindImage {
				t.Errorf("List(%q) matched an image on a single letter", needle)
			}
		}
	}
}

// A failed blob write rolls the entry back. The entries evicted to make room
// for it are already out of the index, so their blobs have to go too --
// otherwise nothing will ever reference or clean them up again.
func TestFailedBlobWriteDoesNotOrphanEvictedBlobs(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	first, _, err := s.Add(Capture{
		Kind:  KindImage,
		Image: pngBytes(t, 2, 2, color.RGBA{255, 0, 0, 255}),
		At:    time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	blobs := filepath.Join(dir, "blobs")
	evictedBlob := filepath.Join(blobs, first.ImageFile)

	// Make the *next* blob path a non-empty directory so the atomic rename onto
	// it fails. The blob directory itself stays writable, so the rollback can
	// still clean up -- a read-only directory would block that too and prove
	// nothing.
	var n int
	if _, err := fmt.Sscanf(first.ID, "i%d", &n); err != nil {
		t.Fatalf("unexpected ID format %q: %v", first.ID, err)
	}
	blocker := filepath.Join(blobs, fmt.Sprintf("i%d.png", n+1))
	if err := os.MkdirAll(filepath.Join(blocker, "occupied"), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Add(Capture{
		Kind:  KindImage,
		Image: pngBytes(t, 3, 3, color.RGBA{0, 255, 0, 255}),
		At:    time.Now().Add(time.Second),
	}); err == nil {
		t.Fatal("the blob write should have failed; the test cannot exercise rollback")
	}

	if _, err := os.Stat(evictedBlob); !os.IsNotExist(err) {
		t.Error("the evicted entry's blob was left behind by the rollback path")
	}
}

func TestDeleteAndClear(t *testing.T) {
	s := newStore(t, 10)
	base := time.Now()

	a, _ := addText(t, s, "a", base)
	addText(t, s, "b", base.Add(time.Second))

	if err := s.Delete(a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if s.Count() != 1 {
		t.Errorf("Count after delete = %d, want 1", s.Count())
	}
	if err := s.Delete("missing"); err == nil {
		t.Error("deleting a missing entry should error")
	}

	s.Clear()
	if s.Count() != 0 {
		t.Errorf("Count after clear = %d, want 0", s.Count())
	}
}

func pngBytes(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestImageBlobLifecycle(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(dir, 10)
	if err != nil {
		t.Fatal(err)
	}

	data := pngBytes(t, 4, 4, color.RGBA{255, 0, 0, 255})
	it, isNew, err := s.Add(Capture{
		Kind: KindImage, Image: data, ImageW: 4, ImageH: 4, At: time.Now(),
	})
	if err != nil || !isNew {
		t.Fatalf("Add image: %v isNew=%v", err, isNew)
	}

	blob := filepath.Join(dir, "blobs", it.ImageFile)
	if _, err := os.Stat(blob); err != nil {
		t.Fatalf("blob not written: %v", err)
	}

	got, err := s.ImageBytes(it.ID)
	if err != nil {
		t.Fatalf("ImageBytes: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Error("ImageBytes returned different bytes than were stored")
	}

	// Deleting the entry must remove the blob so the directory cannot grow
	// without bound.
	if err := s.Delete(it.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(blob); !os.IsNotExist(err) {
		t.Error("blob should be deleted with its entry")
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()

	s, err := OpenAt(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().Truncate(time.Millisecond)
	addText(t, s, "persisted", base)
	pinned, _ := addText(t, s, "pinned-entry", base.Add(time.Second))
	if _, err := s.TogglePin(pinned.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenAt(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	if reopened.Count() != 2 {
		t.Fatalf("Count after reopen = %d, want 2", reopened.Count())
	}
	list := reopened.List("")
	if !list[0].Pinned || list[0].Text != "pinned-entry" {
		t.Errorf("pin state lost across restart: %+v", list[0])
	}
}

// An index referencing a blob that no longer exists must not resurrect a broken
// entry.
func TestOpenDropsEntriesWithMissingBlobs(t *testing.T) {
	dir := t.TempDir()

	s, err := OpenAt(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	it, _, err := s.Add(Capture{
		Kind: KindImage, Image: pngBytes(t, 2, 2, color.Black), ImageW: 2, ImageH: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	addText(t, s, "text-survives", time.Now())
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(dir, "blobs", it.ImageFile)); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenAt(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	if reopened.Count() != 1 {
		t.Errorf("Count = %d, want 1 (orphan dropped)", reopened.Count())
	}
	if _, ok := reopened.Get(it.ID); ok {
		t.Error("entry with missing blob should have been dropped")
	}
}

// App icons must be stored once per app, not repeated on every entry, and must
// not appear in the history index at all.
func TestSourceIconsAreDedupedAndNotInIndex(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(dir, 50)
	if err != nil {
		t.Fatal(err)
	}

	const icon = "data:image/png;base64,AAAAicon"
	base := time.Now()
	for i := 0; i < 5; i++ {
		if _, _, err := s.Add(Capture{
			Kind: KindText, Text: fmt.Sprintf("entry-%d", i),
			SourceApp: "Chrome", SourceIconKey: "com.google.Chrome", SourceIcon: icon,
			At: base.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	indexRaw, err := os.ReadFile(filepath.Join(dir, "history.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(indexRaw), icon) {
		t.Error("history index contains inline icon data; it should live in icons.json")
	}

	iconsRaw, err := os.ReadFile(filepath.Join(dir, "icons.json"))
	if err != nil {
		t.Fatalf("icons.json not written: %v", err)
	}
	var icons map[string]string
	if err := json.Unmarshal(iconsRaw, &icons); err != nil {
		t.Fatal(err)
	}
	if len(icons) != 1 || icons["com.google.Chrome"] != icon {
		t.Errorf("icons.json = %v, want a single Chrome entry", icons)
	}

	// Reading entries back must still surface the icon to callers.
	reopened, err := OpenAt(dir, 50)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	for _, it := range reopened.List("") {
		if it.SourceIcon != icon {
			t.Fatalf("entry %s lost its icon: %q", it.ID, it.SourceIcon)
		}
	}
}

// Icons for apps no longer in the history should not accumulate forever.
func TestUnusedIconsArePruned(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(dir, 50)
	if err != nil {
		t.Fatal(err)
	}

	item, _, err := s.Add(Capture{
		Kind: KindText, Text: "gone soon",
		SourceApp: "Old", SourceIconKey: "com.old.app", SourceIcon: "data:image/png;base64,OLD",
		At: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Add(Capture{
		Kind: KindText, Text: "stays",
		SourceApp: "New", SourceIconKey: "com.new.app", SourceIcon: "data:image/png;base64,NEW",
		At: time.Now().Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(item.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "icons.json"))
	if err != nil {
		t.Fatal(err)
	}
	var icons map[string]string
	if err := json.Unmarshal(raw, &icons); err != nil {
		t.Fatal(err)
	}
	if _, ok := icons["com.old.app"]; ok {
		t.Error("icon for a deleted entry's app should have been pruned")
	}
	if _, ok := icons["com.new.app"]; !ok {
		t.Error("icon still in use was pruned")
	}
}

// Histories written before icons were extracted stored them inline; those must
// be migrated rather than lost.
func TestMigratesInlineIconsFromOlderIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "blobs"), 0o700); err != nil {
		t.Fatal(err)
	}

	const icon = "data:image/png;base64,LEGACY"
	legacy := `[{"id":"i1","kind":"text","text":"old entry","hash":"abc",
	  "sourceApp":"Safari","sourceIcon":"` + icon + `",
	  "firstCopy":"2026-01-01T00:00:00Z","lastCopy":"2026-01-01T00:00:00Z",
	  "copyCount":1,"pinned":false}]`

	if err := os.WriteFile(filepath.Join(dir, "history.json"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := OpenAt(dir, 50)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	items := s.List("")
	if len(items) != 1 {
		t.Fatalf("got %d entries, want 1", len(items))
	}
	if items[0].SourceIcon != icon {
		t.Errorf("migrated icon = %q, want %q", items[0].SourceIcon, icon)
	}

	// And it should now be written to the shared file, not back into the index.
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	indexRaw, _ := os.ReadFile(filepath.Join(dir, "history.json"))
	if strings.Contains(string(indexRaw), icon) {
		t.Error("index still contains the inline icon after migration")
	}
}

func TestPreview(t *testing.T) {
	cases := []struct {
		name string
		item Item
		want string
	}{
		{"single line", Item{Kind: KindText, Text: "hello"}, "hello"},
		{"collapses newlines", Item{Kind: KindText, Text: "a\n\nb\tc"}, "a b c"},
		{"trims", Item{Kind: KindText, Text: "  padded  "}, "padded"},
		{"whitespace only", Item{Kind: KindText, Text: "   \n\t"}, "(whitespace)"},
		{"image", Item{Kind: KindImage}, "Image"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.item.Preview(); got != tc.want {
				t.Errorf("Preview() = %q, want %q", got, tc.want)
			}
		})
	}
}
