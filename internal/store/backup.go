package store

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxImportItems bounds one import. History is capped at 2000 unpinned entries
// plus pins, so a larger file is either hand-made or hostile.
const MaxImportItems = 10000

// maxIconBytes bounds one imported icon data URL. Captured icons are 32px PNGs
// of a few kilobytes; anything far larger is not one of ours.
const maxIconBytes = 256 << 10

const pngDataURLPrefix = "data:image/png;base64,"

// ExportItems returns detached copies of the history for a backup, oldest
// state first as stored. Image entries are left out unless includeImages is
// set, because their thumbnail is image content too. File bookmarks are
// cleared: they are access grants for this machine's sandbox and mean nothing
// elsewhere.
func (s *Store) ExportItems(includeImages bool) ([]Item, map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Item, 0, len(s.items))
	icons := make(map[string]string)
	for _, it := range s.items {
		if it.Kind == KindImage && !includeImages {
			continue
		}
		c := *it
		// The thumbnail is rebuilt from the full image on restore.
		c.Thumb = ""
		c.SourceIcon = ""
		c.TextChars, c.TextLines = 0, 0
		if len(it.Files) > 0 {
			c.Files = append([]FileReference(nil), it.Files...)
			for idx := range c.Files {
				c.Files[idx].Bookmark = ""
				c.Files[idx].Missing = false
			}
		}
		items = append(items, c)
		if icon, ok := s.icons[it.SourceIconKey]; ok && it.SourceIconKey != "" {
			icons[it.SourceIconKey] = icon
		}
	}
	return items, icons
}

// BlobPath returns where the full-size image of an exported entry lives.
func (s *Store) BlobPath(imageFile string) string {
	return filepath.Join(s.blobDir, filepath.Base(imageFile))
}

// ImportImage is the decoded payload of an imported image entry. The caller
// validates the PNG and derives the thumbnail; the store only owns placement.
type ImportImage struct {
	Path  string // staged PNG, renamed into the blob directory when added
	Thumb string
	W, H  int
}

// MergeResult reports what an import changed.
type MergeResult struct {
	Added   int `json:"added"`
	Merged  int `json:"merged"`
	Skipped int `json:"skipped"`
	Evicted int `json:"evicted"`
}

type importCandidate struct {
	item  Item
	image *ImportImage
}

// Merge folds imported entries into the history. An entry whose payload is
// already present keeps the existing record and takes the earlier first copy,
// the later last copy and use, the larger copy count and the pin, so importing
// the same backup twice changes nothing the second time. New entries receive
// fresh IDs. Hashes are recomputed from the payload rather than trusted.
//
// images maps an imported entry's ImageFile to its staged, validated payload;
// image entries without one are skipped. Staged files must be on the same
// volume as the history (the caller stages them inside the data directory) so
// they can be renamed into place; files not moved remain the caller's to
// delete.
func (s *Store) Merge(items []Item, icons map[string]string, images map[string]ImportImage) (MergeResult, error) {
	var result MergeResult
	if len(items) > MaxImportItems {
		return result, errors.New("backup contains too many history entries")
	}

	candidates := make([]importCandidate, 0, len(items))
	for _, it := range items {
		candidate, ok := prepareImport(it, images)
		if !ok {
			result.Skipped++
			continue
		}
		candidates = append(candidates, candidate)
	}

	// Move staged images into the blob directory before taking the write lock,
	// so a large restore does not stall clipboard capture behind disk I/O. Each
	// new image gets the name its entry will own if it is added.
	s.mu.RLock()
	existing := make(map[string]bool, len(s.items))
	for _, it := range s.items {
		existing[it.Hash] = true
	}
	s.mu.RUnlock()

	var placed []string
	for idx := range candidates {
		c := &candidates[idx]
		if c.image == nil || existing[c.item.Hash] {
			continue
		}
		s.mu.Lock()
		c.item.ID = s.newID()
		s.mu.Unlock()
		c.item.ImageFile = c.item.ID + ".png"
		dest := filepath.Join(s.blobDir, c.item.ImageFile)
		if err := os.Rename(c.image.Path, dest); err != nil {
			for _, path := range placed {
				os.Remove(path)
			}
			return MergeResult{}, err
		}
		placed = append(placed, dest)
		// A later entry with the same payload merges into this one below
		// instead of renaming a staged file that is already gone.
		existing[c.item.Hash] = true
	}

	s.mu.Lock()
	byHash := make(map[string]*Item, len(s.items))
	nextPriority := 0
	for _, it := range s.items {
		byHash[it.Hash] = it
		if it.Pinned && it.PinPriority > nextPriority {
			nextPriority = it.PinPriority
		}
	}

	// Imported manual pin order follows the existing manual order.
	sort.SliceStable(candidates, func(i, j int) bool {
		pi, pj := candidates[i].item.PinPriority, candidates[j].item.PinPriority
		if (pi > 0) != (pj > 0) {
			return pi > 0
		}
		return pi < pj
	})

	var orphaned []string
	for idx := range candidates {
		imported := candidates[idx].item
		if current, ok := byHash[imported.Hash]; ok {
			mergeInto(current, &imported, &nextPriority)
			result.Merged++
			if imported.ImageFile != "" && candidates[idx].image != nil && imported.ID != "" {
				// Placed above, then matched an entry captured meanwhile.
				orphaned = append(orphaned, filepath.Join(s.blobDir, imported.ImageFile))
			}
			continue
		}
		if candidates[idx].image != nil && imported.ImageFile == "" {
			// The matching entry existed when images were placed and was
			// deleted since, so this one has no blob of its own.
			result.Skipped++
			continue
		}
		item := imported
		if item.ID == "" {
			item.ID = s.newID()
		}
		if item.Pinned && item.PinPriority > 0 {
			nextPriority++
			item.PinPriority = nextPriority
		} else {
			item.PinPriority = 0
		}
		stored := item
		s.items = append(s.items, &stored)
		byHash[stored.Hash] = &stored
		result.Added++
	}

	for key, icon := range icons {
		if key != "" && strings.HasPrefix(icon, pngDataURLPrefix) && len(icon) <= maxIconBytes {
			s.rememberIconLocked(key, icon)
		}
	}

	s.sortItems()
	s.normalisePinPriorities()
	evicted := s.evictLocked()
	result.Evicted = len(evicted)
	s.mu.Unlock()

	for _, path := range orphaned {
		os.Remove(path)
	}
	s.removeBlobs(evicted)
	return result, s.Flush()
}

// prepareImport validates one imported entry and recomputes its dedupe hash.
func prepareImport(it Item, images map[string]ImportImage) (importCandidate, bool) {
	c := importCandidate{}
	item := Item{
		Kind:          it.Kind,
		SourceApp:     it.SourceApp,
		SourceIconKey: it.SourceIconKey,
		FirstCopy:     it.FirstCopy,
		LastCopy:      it.LastCopy,
		LastUsed:      it.LastUsed,
		CopyCount:     it.CopyCount,
		Pinned:        it.Pinned,
		PinPriority:   it.PinPriority,
	}
	if item.LastCopy.IsZero() {
		return c, false
	}
	if item.FirstCopy.IsZero() || item.FirstCopy.After(item.LastCopy) {
		item.FirstCopy = item.LastCopy
	}
	if item.CopyCount < 1 {
		item.CopyCount = 1
	}
	if !item.Pinned {
		item.PinPriority = 0
	}

	switch it.Kind {
	case KindText:
		if it.Text == "" {
			return c, false
		}
		item.Text = it.Text
		item.Bytes = int64(len(it.Text))
		item.Hash = hashText(it.Text)
	case KindImage:
		staged, ok := images[it.ImageFile]
		if !ok || it.ImageFile == "" {
			return c, false
		}
		hash, size, err := hashBlobFile(staged.Path)
		if err != nil || size == 0 {
			return c, false
		}
		item.Hash = hash
		item.Thumb = staged.Thumb
		item.ImageW, item.ImageH = staged.W, staged.H
		item.Bytes = size
		c.image = &staged
	case KindFile:
		if len(it.Files) == 0 {
			return c, false
		}
		files := make([]FileReference, 0, len(it.Files))
		var total int64
		for _, file := range it.Files {
			if file.Path == "" {
				return c, false
			}
			path := filepath.Clean(file.Path)
			files = append(files, FileReference{
				Path:      path,
				Name:      filepath.Base(path),
				Directory: file.Directory,
				Bytes:     file.Bytes,
			})
			total += file.Bytes
		}
		item.Files = files
		item.FileCount = len(files)
		item.Bytes = total
		item.Hash = hashFiles(files)
	default:
		return c, false
	}
	c.item = item
	return c, true
}

func mergeInto(current, imported *Item, nextPriority *int) {
	if imported.FirstCopy.Before(current.FirstCopy) {
		current.FirstCopy = imported.FirstCopy
	}
	current.LastCopy = laterOf(current.LastCopy, imported.LastCopy)
	current.LastUsed = laterOf(current.LastUsed, imported.LastUsed)
	if imported.CopyCount > current.CopyCount {
		current.CopyCount = imported.CopyCount
	}
	if current.SourceApp == "" && imported.SourceApp != "" {
		current.SourceApp = imported.SourceApp
		current.SourceIconKey = imported.SourceIconKey
	}
	if imported.Pinned && !current.Pinned {
		current.Pinned = true
		if imported.PinPriority > 0 {
			*nextPriority++
			current.PinPriority = *nextPriority
		}
	}
}

// hashBlobFile is hashBytes over a file, streamed so a restore holds one
// buffer rather than every image at once.
func hashBlobFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := newBlobHash()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return blobHashSum(h), n, nil
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
