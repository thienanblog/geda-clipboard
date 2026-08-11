// Command screenshot-fixture writes a synthetic clipboard history into a
// throwaway home directory, so the App Store screenshots can show a full,
// believable list without putting anybody's real clipboard on the store page.
//
// It builds the history through internal/store rather than by writing JSON, so
// the hashes, IDs, blob filenames and index layout are produced by the same
// code the app uses. A fixture hand-rolled as JSON drifts from the schema the
// first time the schema moves, and the failure is a blank popup five minutes
// before a submission.
//
// Usage:
//
//	go run ./scripts/screenshot-fixture -home /tmp/geda-shots
//	HOME=/tmp/geda-shots wails dev
//
// The app resolves its data directory from HOME, so pointing HOME at the
// fixture is all it takes. Nothing here touches the real history.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"geda-clipboard/internal/clipboard"
	"geda-clipboard/internal/imageutil"
	"geda-clipboard/internal/settings"
	"geda-clipboard/internal/store"
)

// entry is one line of the fixture. Ago places it in the past; the store sorts
// on that, so the order here is oldest first and the list reads newest first.
type entry struct {
	Text   string
	App    string
	Bundle string
	Ago    time.Duration
	Pin    bool
	// Repeat adds the same payload again, which is how the store records a
	// re-copy: the entry keeps its place in the index and its counter goes up.
	Repeat int
}

// Source apps are macOS built-ins on purpose. A clipboard manager's screenshots
// have to show provenance to make sense, and inventing app names would look
// exactly as false as it would be.
var fixture = []entry{
	{
		Text:   "https://developer.apple.com/design/human-interface-guidelines/",
		App:    "Safari",
		Bundle: "com.apple.Safari",
		Ago:    52 * time.Minute,
	},
	{
		Text:   "Meeting moved to Thursday 10:00. Agenda: release checklist, then the localisation pass.",
		App:    "Mail",
		Bundle: "com.apple.mail",
		Ago:    47 * time.Minute,
	},
	{
		Text:   "#2E7DF7",
		App:    "Notes",
		Bundle: "com.apple.Notes",
		Ago:    41 * time.Minute,
		Repeat: 2,
	},
	{
		Text:   "git rebase --interactive origin/main",
		App:    "Terminal",
		Bundle: "com.apple.Terminal",
		Ago:    36 * time.Minute,
		Pin:    true,
	},
	{
		Text:   "func (s *Store) List(query string) []*Item {\n\ts.mu.RLock()\n\tdefer s.mu.RUnlock()\n\treturn s.filter(query)\n}",
		App:    "Terminal",
		Bundle: "com.apple.Terminal",
		Ago:    31 * time.Minute,
	},
	{
		Text:   "{\"id\":\"ord_8412\",\"status\":\"shipped\",\"items\":3,\"total\":\"48.00\"}",
		App:    "Safari",
		Bundle: "com.apple.Safari",
		Ago:    27 * time.Minute,
	},
	{
		Text:   "Invoice 2026-0184 — due 30 September",
		App:    "Preview",
		Bundle: "com.apple.Preview",
		Ago:    22 * time.Minute,
	},
	{
		Text:   "support@example.com",
		App:    "Mail",
		Bundle: "com.apple.mail",
		Ago:    18 * time.Minute,
		Pin:    true,
		Repeat: 3,
	},
	{
		Text:   "docker compose up --build --detach",
		App:    "Terminal",
		Bundle: "com.apple.Terminal",
		Ago:    14 * time.Minute,
	},
	{
		Text:   "The second draft reads better with the middle section cut. Keep the opening.",
		App:    "Notes",
		Bundle: "com.apple.Notes",
		Ago:    11 * time.Minute,
	},
	{
		Text:   "SELECT id, name FROM customers WHERE created_at > now() - interval '7 days';",
		App:    "Terminal",
		Bundle: "com.apple.Terminal",
		Ago:    7 * time.Minute,
	},
	{
		Text:   "1 Infinite Loop, Cupertino, CA 95014",
		App:    "Notes",
		Bundle: "com.apple.Notes",
		Ago:    5 * time.Minute,
	},
	{
		Text:   "Reviewed and approved — ship it.",
		App:    "Mail",
		Bundle: "com.apple.mail",
		Ago:    2 * time.Minute,
	},
}

func main() {
	home := flag.String("home", "", "throwaway home directory to write the fixture into")
	flag.Parse()

	if *home == "" {
		log.Fatal("-home is required; point it somewhere disposable, never at your real home")
	}
	abs, err := filepath.Abs(*home)
	if err != nil {
		log.Fatal(err)
	}
	if abs == os.Getenv("HOME") {
		log.Fatal("-home is your real home directory; refusing to overwrite the real history")
	}

	dir := filepath.Join(abs, "Library", "Application Support", "geda-clipboard")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Fatal(err)
	}

	// Start from empty so repeated runs are identical rather than cumulative.
	for _, name := range []string{"history.json", "icons.json", "settings.json"} {
		os.Remove(filepath.Join(dir, name))
	}
	os.RemoveAll(filepath.Join(dir, "blobs"))

	st, err := store.OpenAt(dir, settings.Defaults().MaxItems)
	if err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	icons := map[string]string{}
	iconFor := func(bundleID string) string {
		if url, ok := icons[bundleID]; ok {
			return url
		}
		url := imageutil.DataURL(clipboard.AppIconPNG(bundleID, 32))
		icons[bundleID] = url
		return url
	}

	var pins []string
	for _, e := range fixture {
		capture := store.Capture{
			Kind:          store.KindText,
			Text:          e.Text,
			SourceApp:     e.App,
			SourceIconKey: e.Bundle,
			SourceIcon:    iconFor(e.Bundle),
			At:            now.Add(-e.Ago),
		}

		item, _, err := st.Add(capture)
		if err != nil {
			log.Fatalf("add %q: %v", e.Text, err)
		}
		// Re-adding the identical payload is what a real re-copy looks like, so
		// the counter in the detail card is produced rather than faked.
		for i := 0; i < e.Repeat; i++ {
			capture.At = now.Add(-e.Ago).Add(time.Duration(i+1) * time.Minute)
			if item, _, err = st.Add(capture); err != nil {
				log.Fatalf("repeat %q: %v", e.Text, err)
			}
		}
		if e.Pin {
			pins = append(pins, item.ID)
		}
	}

	// One image entry, so the list shows a thumbnail row and the detail card has
	// something to enlarge. Drawn here rather than shipped as a file: a binary
	// asset in git for one screenshot is a poor trade, and this way the size and
	// the thumbnail bounds come out of the real pipeline.
	shot := syntheticImage(1200, 750)
	thumb, w, h, err := imageutil.Thumbnail(shot, 360, 150)
	if err != nil {
		log.Fatal(err)
	}
	if _, _, err := st.Add(store.Capture{
		Kind:          store.KindImage,
		Image:         shot,
		Thumb:         thumb,
		ImageW:        w,
		ImageH:        h,
		SourceApp:     "Preview",
		SourceIconKey: "com.apple.Preview",
		SourceIcon:    iconFor("com.apple.Preview"),
		At:            now.Add(-25 * time.Minute),
	}); err != nil {
		log.Fatal(err)
	}

	for _, id := range pins {
		if _, err := st.TogglePin(id); err != nil {
			log.Fatalf("pin %s: %v", id, err)
		}
	}

	if err := st.Close(); err != nil {
		log.Fatal(err)
	}

	// Preferences the screenshots should show, rather than whatever the defaults
	// happen to be: previews on, so the detail card can be captured beside the
	// list, and the menu bar placement, which is what a still image can explain.
	cfg := settings.Defaults()
	cfg.PopupPlacement = settings.PlacementMenuBar
	cfg.PreviewOnHover = true
	if err := writeSettings(dir, cfg); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Wrote %d entries to %s\n", st.Count(), dir)
	fmt.Printf("Run the app against it with:\n  HOME=%s wails dev\n", abs)
}

func writeSettings(dir string, cfg settings.Settings) error {
	// Written through the settings package for the same reason the history is:
	// validation and the layout stamp are applied by the code that owns them.
	old := os.Getenv("HOME")
	home := filepath.Clean(filepath.Join(dir, "..", "..", ".."))
	if err := os.Setenv("HOME", home); err != nil {
		return err
	}
	defer os.Setenv("HOME", old)

	m, err := settings.Load()
	if err != nil {
		return err
	}
	_, err = m.Save(cfg)
	return err
}

// syntheticImage draws a placeholder that reads as a captured screenshot at
// thumbnail size: a soft diagonal gradient with a lighter card over it.
func syntheticImage(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t := (float64(x)/float64(w) + float64(y)/float64(h)) / 2
			img.Set(x, y, color.RGBA{
				R: uint8(28 + 40*t),
				G: uint8(46 + 70*t),
				B: uint8(96 + 120*t),
				A: 255,
			})
		}
	}

	card := image.Rect(w/8, h/5, w-w/8, h-h/5)
	draw.Draw(img, card, &image.Uniform{color.RGBA{246, 248, 252, 255}}, image.Point{}, draw.Src)

	// A few bars inside the card, so the thumbnail is not a flat rectangle.
	for i := 0; i < 5; i++ {
		top := card.Min.Y + 40 + i*46
		width := int(float64(card.Dx()-80) * (0.9 - 0.12*math.Abs(float64(i)-1.5)))
		bar := image.Rect(card.Min.X+40, top, card.Min.X+40+width, top+18)
		shade := uint8(190 - i*18)
		draw.Draw(img, bar, &image.Uniform{color.RGBA{shade, shade + 10, 220, 255}}, image.Point{}, draw.Src)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}
