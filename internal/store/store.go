// Package store keeps the clipboard history in memory and persists it to disk.
//
// History is small and bounded (a few hundred entries), so it is held as a
// slice and filtered in memory. Image payloads live as separate blob files so
// the index file stays small and cheap to rewrite.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"geda-clipboard/internal/appdir"
)

// Capture is an incoming clipboard event to be recorded.
type Capture struct {
	Kind   Kind
	Text   string
	Image  []byte // PNG bytes for KindImage
	ImageW int
	ImageH int
	Thumb  string // PNG data URL

	SourceApp string
	// SourceIconKey identifies the source app (bundle ID or executable path)
	// and SourceIcon is its icon as a PNG data URL. The icon is stored once per
	// key rather than on every entry.
	SourceIconKey string
	SourceIcon    string

	At time.Time
}

// Store is the clipboard history. All methods are safe for concurrent use.
type Store struct {
	mu    sync.RWMutex
	items []*Item // newest first, ignoring pin state

	indexPath string
	iconsPath string
	blobDir   string

	// icons maps a source-app key to its icon data URL. Kept out of the history
	// index so a repeated app icon costs one copy, not one per entry.
	icons map[string]string

	maxItems int

	// save coalescing
	saveMu      sync.Mutex
	pendingSave bool
	saveTimer   *time.Timer

	nextID int64
}

// Open loads the history from the user's application data directory.
func Open(maxItems int) (*Store, error) {
	dir, err := appdir.Data()
	if err != nil {
		return nil, err
	}
	if _, err := appdir.Blobs(); err != nil {
		return nil, err
	}
	return OpenAt(dir, maxItems)
}

// OpenAt loads the history from an explicit directory. A missing or corrupt
// index yields an empty history rather than an error, so the app always starts.
func OpenAt(dir string, maxItems int) (*Store, error) {
	blobs := filepath.Join(dir, "blobs")
	if err := os.MkdirAll(blobs, 0o700); err != nil {
		return nil, err
	}

	s := &Store{
		indexPath: filepath.Join(dir, "history.json"),
		iconsPath: filepath.Join(dir, "icons.json"),
		blobDir:   blobs,
		icons:     make(map[string]string),
		maxItems:  maxItems,
	}

	if iconRaw, err := os.ReadFile(s.iconsPath); err == nil {
		// A corrupt icon cache costs only missing icons, so ignore errors.
		json.Unmarshal(iconRaw, &s.icons)
	}

	raw, err := os.ReadFile(s.indexPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return s, nil
	}

	var items []*Item
	if err := json.Unmarshal(raw, &items); err != nil {
		return s, nil
	}

	// Drop entries whose blob has gone missing, and re-derive the ID counter.
	for _, it := range items {
		if it == nil || it.Hash == "" {
			continue
		}
		if it.Kind == KindImage {
			if it.ImageFile == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(s.blobDir, it.ImageFile)); err != nil {
				continue
			}
		}

		// Older files stored the icon inline on every entry. Lift it into the
		// shared map so it is written once from now on.
		if it.SourceIcon != "" {
			key := it.SourceIconKey
			if key == "" {
				key = it.SourceApp
				it.SourceIconKey = key
			}
			if key != "" {
				if _, ok := s.icons[key]; !ok {
					s.icons[key] = it.SourceIcon
				}
			}
			it.SourceIcon = ""
		}

		s.items = append(s.items, it)
	}
	s.sortItems()
	s.nextID = time.Now().UnixNano()
	return s, nil
}

// SetMaxItems updates the cap and evicts immediately if needed.
func (s *Store) SetMaxItems(n int) {
	s.mu.Lock()
	s.maxItems = n
	evicted := s.evictLocked()
	s.mu.Unlock()

	s.removeBlobs(evicted)
	if len(evicted) > 0 {
		s.scheduleSave()
	}
}

// sortItems orders newest-first by last copy time. Pin ordering is applied at
// read time in List so unpinning restores chronological position.
func (s *Store) sortItems() {
	sort.SliceStable(s.items, func(i, j int) bool {
		return s.items[i].LastCopy.After(s.items[j].LastCopy)
	})
}

func (s *Store) newID() string {
	s.nextID++
	return fmt.Sprintf("i%d", s.nextID)
}

// Add records a capture. When the payload matches an existing entry, that entry
// is bumped to the front and its copy count incremented instead of being
// duplicated. The returned bool reports whether this was a brand new entry.
func (s *Store) Add(c Capture) (*Item, bool, error) {
	if c.At.IsZero() {
		c.At = time.Now()
	}

	var hash string
	switch c.Kind {
	case KindText:
		if c.Text == "" {
			return nil, false, errors.New("empty text capture")
		}
		hash = hashText(c.Text)
	case KindImage:
		if len(c.Image) == 0 {
			return nil, false, errors.New("empty image capture")
		}
		hash = hashBytes(c.Image)
	default:
		return nil, false, fmt.Errorf("unknown kind %q", c.Kind)
	}

	s.mu.Lock()

	s.rememberIconLocked(c.SourceIconKey, c.SourceIcon)

	// Existing payload: bump it.
	for idx, it := range s.items {
		if it.Hash != hash {
			continue
		}
		it.LastCopy = c.At
		it.CopyCount++
		// Refresh provenance: the most recent copy is the more useful one.
		if c.SourceApp != "" {
			it.SourceApp = c.SourceApp
			it.SourceIconKey = c.SourceIconKey
		}
		// Move to front.
		s.items = append(s.items[:idx], s.items[idx+1:]...)
		s.items = append([]*Item{it}, s.items...)
		copied := s.resolveLocked(it)
		s.mu.Unlock()

		s.scheduleSave()
		return &copied, false, nil
	}

	item := &Item{
		ID:            s.newID(),
		Kind:          c.Kind,
		Hash:          hash,
		SourceApp:     c.SourceApp,
		SourceIconKey: c.SourceIconKey,
		FirstCopy:     c.At,
		LastCopy:      c.At,
		CopyCount:     1,
	}

	switch c.Kind {
	case KindText:
		item.Text = c.Text
		item.Bytes = len(c.Text)
	case KindImage:
		item.ImageFile = item.ID + ".png"
		item.Thumb = c.Thumb
		item.ImageW = c.ImageW
		item.ImageH = c.ImageH
		item.Bytes = len(c.Image)
	}

	s.items = append([]*Item{item}, s.items...)
	evicted := s.evictLocked()
	copied := s.resolveLocked(item)
	blobDir := s.blobDir
	s.mu.Unlock()

	// Write the blob outside the lock.
	if c.Kind == KindImage {
		path := filepath.Join(blobDir, item.ImageFile)
		if err := appdir.WriteAtomic(path, c.Image, 0o600); err != nil {
			// Roll the entry back rather than keeping a dangling reference.
			s.Delete(item.ID)
			return nil, false, fmt.Errorf("write image blob: %w", err)
		}
	}

	s.removeBlobs(evicted)
	s.scheduleSave()
	return &copied, true, nil
}

// rememberIconLocked records an app icon once. Caller must hold the lock.
func (s *Store) rememberIconLocked(key, icon string) {
	if key == "" || icon == "" {
		return
	}
	if _, ok := s.icons[key]; !ok {
		s.icons[key] = icon
	}
}

// resolveLocked returns a copy of it with its source icon filled in, ready to
// hand to the frontend. Caller must hold the lock.
func (s *Store) resolveLocked(it *Item) Item {
	c := *it
	if c.SourceIconKey != "" {
		c.SourceIcon = s.icons[c.SourceIconKey]
	}
	return c
}

// evictLocked trims unpinned entries beyond the cap. Caller must hold the lock.
// Returns the evicted entries so their blobs can be deleted outside the lock.
func (s *Store) evictLocked() []*Item {
	if s.maxItems <= 0 {
		return nil
	}

	var kept []*Item
	var evicted []*Item
	unpinned := 0
	for _, it := range s.items {
		if it.Pinned {
			kept = append(kept, it)
			continue
		}
		if unpinned < s.maxItems {
			unpinned++
			kept = append(kept, it)
			continue
		}
		evicted = append(evicted, it)
	}
	s.items = kept
	return evicted
}

func (s *Store) removeBlobs(items []*Item) {
	for _, it := range items {
		if it.Kind == KindImage && it.ImageFile != "" {
			os.Remove(filepath.Join(s.blobDir, it.ImageFile))
		}
	}
}

// List returns entries matching query, pinned entries first, each already a
// copy safe to hand to the frontend. A blank query returns everything.
func (s *Store) List(query string) []*Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	needle := strings.ToLower(strings.TrimSpace(query))

	var pinned, rest []*Item
	for _, it := range s.items {
		if needle != "" && !matches(it, needle) {
			continue
		}
		c := s.resolveLocked(it)
		if it.Pinned {
			pinned = append(pinned, &c)
		} else {
			rest = append(rest, &c)
		}
	}
	return append(pinned, rest...)
}

func matches(it *Item, needle string) bool {
	if strings.Contains(strings.ToLower(it.Text), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(it.SourceApp), needle) {
		return true
	}
	if it.Kind == KindImage && strings.Contains("image", needle) {
		return true
	}
	return false
}

// Get returns a copy of the entry with the given ID.
func (s *Store) Get(id string) (*Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, it := range s.items {
		if it.ID == id {
			c := s.resolveLocked(it)
			return &c, true
		}
	}
	return nil, false
}

// ImageBytes returns the full-size PNG for an image entry.
func (s *Store) ImageBytes(id string) ([]byte, error) {
	s.mu.RLock()
	var file string
	for _, it := range s.items {
		if it.ID == id && it.Kind == KindImage {
			file = it.ImageFile
			break
		}
	}
	blobDir := s.blobDir
	s.mu.RUnlock()

	if file == "" {
		return nil, errors.New("not an image entry")
	}
	return os.ReadFile(filepath.Join(blobDir, file))
}

// TogglePin flips the pinned flag and reports the new state.
func (s *Store) TogglePin(id string) (bool, error) {
	s.mu.Lock()
	var pinned bool
	found := false
	for _, it := range s.items {
		if it.ID == id {
			it.Pinned = !it.Pinned
			pinned = it.Pinned
			found = true
			break
		}
	}
	var evicted []*Item
	if found {
		// Unpinning can push the entry past the cap.
		evicted = s.evictLocked()
	}
	s.mu.Unlock()

	if !found {
		return false, errors.New("entry not found")
	}
	s.removeBlobs(evicted)
	s.scheduleSave()
	return pinned, nil
}

// Delete removes a single entry.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	var removed *Item
	for idx, it := range s.items {
		if it.ID == id {
			removed = it
			s.items = append(s.items[:idx], s.items[idx+1:]...)
			break
		}
	}
	s.mu.Unlock()

	if removed == nil {
		return errors.New("entry not found")
	}
	s.removeBlobs([]*Item{removed})
	s.scheduleSave()
	return nil
}

// Clear removes every entry, including pinned ones.
func (s *Store) Clear() {
	s.mu.Lock()
	removed := s.items
	s.items = nil
	s.mu.Unlock()

	s.removeBlobs(removed)
	s.scheduleSave()
}

// Count returns the number of stored entries.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// scheduleSave coalesces rapid mutations into a single write a moment later.
func (s *Store) scheduleSave() {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.pendingSave = true
	s.saveTimer = time.AfterFunc(750*time.Millisecond, func() {
		if err := s.Flush(); err != nil {
			fmt.Fprintln(os.Stderr, "geda-clipboard: save history:", err)
		}
	})
}

// Flush writes the history index to disk immediately.
func (s *Store) Flush() error {
	s.saveMu.Lock()
	s.pendingSave = false
	s.saveMu.Unlock()

	s.mu.Lock()
	// Drop icons for apps no longer present in the history, so the cache cannot
	// grow without bound.
	used := make(map[string]string, len(s.icons))
	for _, it := range s.items {
		if key := it.SourceIconKey; key != "" {
			if icon, ok := s.icons[key]; ok {
				used[key] = icon
			}
		}
	}
	s.icons = used

	data, err := json.Marshal(s.items)
	iconData, iconErr := json.Marshal(s.icons)
	s.mu.Unlock()

	if err != nil {
		return err
	}
	if err := appdir.WriteAtomic(s.indexPath, data, 0o600); err != nil {
		return err
	}
	if iconErr != nil {
		return iconErr
	}
	return appdir.WriteAtomic(s.iconsPath, iconData, 0o600)
}

// Close flushes any pending write.
func (s *Store) Close() error {
	s.saveMu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveMu.Unlock()
	return s.Flush()
}
