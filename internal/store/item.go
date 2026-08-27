package store

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Kind distinguishes the payload type of an entry.
type Kind string

const (
	KindText  Kind = "text"
	KindImage Kind = "image"
)

// Item is one clipboard history entry. JSON keys are lowerCamelCase so the
// frontend consumes them directly.
type Item struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`

	// Text is the full text payload for KindText entries.
	Text string `json:"text,omitempty"`

	// ImageFile is the blob filename (not a path) of the full-size PNG, and
	// Thumb is a small inline PNG data URL used for the list. Both apply to
	// KindImage entries only.
	ImageFile string `json:"imageFile,omitempty"`
	Thumb     string `json:"thumb,omitempty"`
	ImageW    int    `json:"imageW,omitempty"`
	ImageH    int    `json:"imageH,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`

	// Hash identifies the payload for dedupe purposes.
	Hash string `json:"hash"`

	// SourceApp is the display name of the app that was frontmost at capture
	// time. SourceIconKey identifies that app (its bundle ID or executable
	// path); the icon itself is held once per app in a separate file rather
	// than repeated in every entry.
	SourceApp     string `json:"sourceApp,omitempty"`
	SourceIconKey string `json:"sourceIconKey,omitempty"`

	// SourceIcon is the app's icon as a PNG data URL. It is resolved when
	// entries are read and deliberately never persisted here -- doing so made
	// the index several megabytes for a history full of images.
	SourceIcon string `json:"sourceIcon,omitempty"`

	FirstCopy time.Time `json:"firstCopy"`
	LastCopy  time.Time `json:"lastCopy"`
	CopyCount int       `json:"copyCount"`

	Pinned bool `json:"pinned"`
	// PinPriority is zero while a pinned entry follows normal recency ordering.
	// Positive values place user-arranged entries first, in ascending order.
	PinPriority int `json:"pinPriority,omitempty"`
}

// Preview returns a single-line label for the entry, suitable for the list.
func (i *Item) Preview() string {
	if i.Kind == KindImage {
		return "Image"
	}
	// Collapse whitespace so multi-line snippets render on one row.
	s := strings.Join(strings.Fields(i.Text), " ")
	if s == "" {
		// All-whitespace payload: describe it rather than showing a blank row.
		return "(whitespace)"
	}
	return s
}

// hashText derives the dedupe key for a text payload.
func hashText(s string) string {
	sum := sha256.Sum256([]byte("t:" + s))
	return hex.EncodeToString(sum[:16])
}

// hashBytes derives the dedupe key for a binary payload.
func hashBytes(b []byte) string {
	h := sha256.New()
	h.Write([]byte("b:"))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil)[:16])
}
