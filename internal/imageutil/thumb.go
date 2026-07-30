// Package imageutil produces the small inline previews shown in the popup list.
package imageutil

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"

	xdraw "golang.org/x/image/draw"
)

// Decoded reports the dimensions of a PNG without keeping the image around.
func Decoded(pngBytes []byte) (w, h int, err error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

// Thumbnail scales a PNG so neither side exceeds maxW/maxH, preserving aspect
// ratio, and returns it as a data URL ready to drop into an <img src>. Images
// already within bounds are re-encoded unchanged.
func Thumbnail(pngBytes []byte, maxW, maxH int) (dataURL string, w, h int, err error) {
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return "", 0, 0, fmt.Errorf("decode png: %w", err)
	}

	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW <= 0 || srcH <= 0 {
		return "", 0, 0, fmt.Errorf("image has zero extent")
	}

	dstW, dstH := fit(srcW, srcH, maxW, maxH)

	var out image.Image
	if dstW == srcW && dstH == srcH {
		out = src
	} else {
		dst := image.NewNRGBA(image.Rect(0, 0, dstW, dstH))
		// CatmullRom keeps text in screenshots legible at small sizes.
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
		out = dst
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, out); err != nil {
		return "", 0, 0, fmt.Errorf("encode thumbnail: %w", err)
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), srcW, srcH, nil
}

// fit returns the largest size within maxW x maxH that preserves the aspect
// ratio of srcW x srcH, never enlarging.
func fit(srcW, srcH, maxW, maxH int) (int, int) {
	if maxW <= 0 || maxH <= 0 {
		return srcW, srcH
	}
	if srcW <= maxW && srcH <= maxH {
		return srcW, srcH
	}

	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}

	w := int(float64(srcW) * scale)
	h := int(float64(srcH) * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// DataURL wraps raw PNG bytes as a data URL without rescaling.
func DataURL(pngBytes []byte) string {
	if len(pngBytes) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)
}
