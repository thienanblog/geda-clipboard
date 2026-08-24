//go:build darwin

package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// preserveClipboard saves the current clipboard and restores it afterwards, so
// running the tests does not destroy whatever the user had copied.
func preserveClipboard(t *testing.T) {
	t.Helper()

	before, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	t.Cleanup(func() {
		switch before.Kind {
		case KindText:
			WriteText(before.Text)
		case KindImage:
			WriteImage(before.Image)
		}
	})
}

func TestChangeCountIncreasesOnWrite(t *testing.T) {
	preserveClipboard(t)

	start := ChangeCount()
	after, err := WriteText("geda-clipboard test payload")
	if err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if after <= start {
		t.Errorf("change counter did not advance: %d -> %d", start, after)
	}
	if got := ChangeCount(); got != after {
		t.Errorf("ChangeCount() = %d, want %d as returned by WriteText", got, after)
	}
}

func TestTextRoundTrip(t *testing.T) {
	preserveClipboard(t)

	const want = "hello from geda-clipboard – ünïcodé ✅"
	if _, err := WriteText(want); err != nil {
		t.Fatalf("WriteText: %v", err)
	}

	snap, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if snap.Kind != KindText {
		t.Fatalf("Kind = %v, want KindText", snap.Kind)
	}
	if snap.Text != want {
		t.Errorf("Text = %q, want %q", snap.Text, want)
	}
}

func TestImageRoundTrip(t *testing.T) {
	preserveClipboard(t)

	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(1, 1, color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteImage(encoded.Bytes()); err != nil {
		t.Fatalf("WriteImage: %v", err)
	}

	snap, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if snap.Kind != KindImage {
		t.Fatalf("Kind = %v, want KindImage", snap.Kind)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(snap.Image))
	if err != nil {
		t.Fatalf("returned image is not PNG: %v", err)
	}
	if cfg.Width != 3 || cfg.Height != 2 {
		t.Errorf("image size = %dx%d, want 3x2", cfg.Width, cfg.Height)
	}
}

// Copying identical content twice must still advance the counter; the whole
// copy-count feature depends on this.
func TestRepeatedIdenticalCopyAdvancesCounter(t *testing.T) {
	preserveClipboard(t)

	first, err := WriteText("same text")
	if err != nil {
		t.Fatal(err)
	}
	second, err := WriteText("same text")
	if err != nil {
		t.Fatal(err)
	}
	if second <= first {
		t.Errorf("counter did not advance on identical re-copy: %d -> %d", first, second)
	}
}

func TestFrontmostReturnsSomething(t *testing.T) {
	app := Frontmost()
	// Under `go test` the frontmost app is whatever the user has focused; we can
	// only assert the call works and yields a plausible identity.
	if app.Name == "" && app.BundleID == "" {
		t.Skip("no frontmost application reported (headless session?)")
	}
	t.Logf("frontmost: name=%q bundleID=%q", app.Name, app.BundleID)
}

func TestAppIconPNG(t *testing.T) {
	// Finder is present on every macOS install.
	icon := AppIconPNG("com.apple.finder", 32)
	if len(icon) == 0 {
		t.Fatal("no icon returned for com.apple.finder")
	}
	if len(icon) < 8 || string(icon[1:4]) != "PNG" {
		t.Errorf("returned data is not a PNG (len=%d, prefix=%q)", len(icon), icon[:min(8, len(icon))])
	}
	t.Logf("finder icon: %d bytes", len(icon))
}

func TestAppIconPNGUnknownBundle(t *testing.T) {
	if icon := AppIconPNG("com.example.definitely-not-installed", 32); icon != nil {
		t.Errorf("expected nil for unknown bundle id, got %d bytes", len(icon))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
