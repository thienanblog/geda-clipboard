package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Kind distinguishes the payload type of an entry.
type Kind string

const (
	KindText  Kind = "text"
	KindImage Kind = "image"
	KindFile  Kind = "file"
)

// FileReference is one filesystem object in a copied file group. Geda keeps
// the path rather than duplicating the object in its data directory. Missing
// is derived for display and is never persisted as authoritative state.
type FileReference struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Directory bool   `json:"directory,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`
	Missing   bool   `json:"missing,omitempty"`
	// Bookmark is the opaque macOS security-scoped bookmark that preserves file
	// access across launches. Copies returned to the frontend always clear it.
	Bookmark string `json:"-"`
}

// MarshalJSON persists Bookmark while keeping it out of Wails' generated
// frontend model. Frontend-facing copies clear the value before encoding, so
// the sandbox access grant never crosses into JavaScript.
func (f FileReference) MarshalJSON() ([]byte, error) {
	type persistedFileReference struct {
		Path      string `json:"path"`
		Name      string `json:"name"`
		Directory bool   `json:"directory,omitempty"`
		Bytes     int64  `json:"bytes,omitempty"`
		Missing   bool   `json:"missing,omitempty"`
		Bookmark  string `json:"bookmark,omitempty"`
	}
	return json.Marshal(persistedFileReference(f))
}

// UnmarshalJSON accepts both new bookmark-bearing entries and older histories
// that stored only paths and metadata.
func (f *FileReference) UnmarshalJSON(data []byte) error {
	type persistedFileReference struct {
		Path      string `json:"path"`
		Name      string `json:"name"`
		Directory bool   `json:"directory,omitempty"`
		Bytes     int64  `json:"bytes,omitempty"`
		Missing   bool   `json:"missing,omitempty"`
		Bookmark  string `json:"bookmark,omitempty"`
	}
	var persisted persistedFileReference
	if err := json.Unmarshal(data, &persisted); err != nil {
		return err
	}
	*f = FileReference(persisted)
	return nil
}

// Item is one clipboard history entry. JSON keys are lowerCamelCase so the
// frontend consumes them directly.
type Item struct {
	ID   string `json:"id"`
	Kind Kind   `json:"kind"`

	// Text is the full text payload for KindText entries.
	Text string `json:"text,omitempty"`
	// TextChars and TextLines are derived only on display copies returned to the
	// frontend. Stored entries leave them at zero, so persistence remains bounded
	// while the detail pane can describe a truncated preview accurately.
	TextChars int `json:"textChars,omitempty"`
	TextLines int `json:"textLines,omitempty"`

	// ImageFile is the blob filename (not a path) of the full-size PNG, and
	// Thumb is a small inline PNG data URL used for the list. Both apply to
	// KindImage entries only.
	ImageFile string `json:"imageFile,omitempty"`
	Thumb     string `json:"thumb,omitempty"`
	ImageW    int    `json:"imageW,omitempty"`
	ImageH    int    `json:"imageH,omitempty"`
	Bytes     int64  `json:"bytes,omitempty"`

	// Files preserves the order of a multi-file copy. FileCount remains on list
	// previews even when Files is reduced to the first item.
	Files     []FileReference `json:"files,omitempty"`
	FileCount int             `json:"fileCount,omitempty"`

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
	if i.Kind == KindFile {
		if len(i.Files) == 0 {
			return "Files"
		}
		if i.FileCount > 1 {
			return i.Files[0].Name + " +" + strconv.Itoa(i.FileCount-1)
		}
		return i.Files[0].Name
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

func hashFiles(files []FileReference) string {
	h := sha256.New()
	h.Write([]byte("f:"))
	for _, file := range files {
		h.Write([]byte(filepath.Clean(file.Path)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}
