//go:build windows

package clipboard

import (
	"encoding/binary"
	"reflect"
	"testing"
	"unicode/utf16"
)

func TestEncodeHDropPreservesOrderedPaths(t *testing.T) {
	want := []string{`C:\one.txt`, `D:\Folder\two.png`}
	payload, err := encodeHDrop(want)
	if err != nil {
		t.Fatal(err)
	}
	if offset := binary.LittleEndian.Uint32(payload[:4]); offset != 20 {
		t.Fatalf("pFiles offset = %d, want 20", offset)
	}
	if wide := binary.LittleEndian.Uint32(payload[16:20]); wide != 1 {
		t.Fatalf("fWide = %d, want 1", wide)
	}

	var got []string
	var current []uint16
	for pos := 20; pos < len(payload); pos += 2 {
		value := binary.LittleEndian.Uint16(payload[pos : pos+2])
		if value != 0 {
			current = append(current, value)
			continue
		}
		if len(current) == 0 {
			break
		}
		got = append(got, string(utf16.Decode(current)))
		current = nil
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded paths = %q, want %q", got, want)
	}
}
