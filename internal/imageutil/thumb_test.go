package imageutil

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFit(t *testing.T) {
	cases := []struct {
		name                         string
		sw, sh, mw, mh, wantW, wantH int
	}{
		{"within bounds is untouched", 100, 50, 640, 480, 100, 50},
		{"wide image limited by width", 1600, 400, 640, 480, 640, 160},
		{"tall image limited by height", 400, 1600, 640, 480, 120, 480},
		{"exact fit", 640, 480, 640, 480, 640, 480},
		{"never enlarges", 10, 10, 640, 480, 10, 10},
		{"extreme aspect keeps at least one pixel", 10000, 1, 640, 480, 640, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, h := fit(tc.sw, tc.sh, tc.mw, tc.mh)
			if w != tc.wantW || h != tc.wantH {
				t.Errorf("fit(%d,%d,%d,%d) = %d,%d; want %d,%d",
					tc.sw, tc.sh, tc.mw, tc.mh, w, h, tc.wantW, tc.wantH)
			}
		})
	}
}

func TestThumbnailScalesDownAndReportsSourceSize(t *testing.T) {
	src := makePNG(t, 1200, 300)

	url, w, h, err := Thumbnail(src, 600, 400)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	// Reported dimensions are the *source* dimensions, which is what the UI
	// displays as the image's real size.
	if w != 1200 || h != 300 {
		t.Errorf("reported size = %dx%d, want 1200x300", w, h)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Fatalf("thumbnail is not a png data URL: %.40q", url)
	}

	// Decode the thumbnail back and check it was actually scaled.
	raw := decodeDataURL(t, url)
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 600 || cfg.Height != 150 {
		t.Errorf("thumbnail size = %dx%d, want 600x150", cfg.Width, cfg.Height)
	}
	if len(raw) >= len(src) {
		t.Errorf("thumbnail (%d bytes) should be smaller than source (%d bytes)", len(raw), len(src))
	}
}

func TestThumbnailRejectsNonPNG(t *testing.T) {
	if _, _, _, err := Thumbnail([]byte("not an image"), 100, 100); err == nil {
		t.Error("expected an error for non-PNG input")
	}
}

func TestDecoded(t *testing.T) {
	w, h, err := Decoded(makePNG(t, 37, 91))
	if err != nil {
		t.Fatal(err)
	}
	if w != 37 || h != 91 {
		t.Errorf("Decoded = %dx%d, want 37x91", w, h)
	}
}

func TestDataURLEmpty(t *testing.T) {
	if got := DataURL(nil); got != "" {
		t.Errorf("DataURL(nil) = %q, want empty", got)
	}
}

func decodeDataURL(t *testing.T, url string) []byte {
	t.Helper()
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("not a png data URL")
	}
	raw, err := base64Decode(url[len(prefix):])
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
