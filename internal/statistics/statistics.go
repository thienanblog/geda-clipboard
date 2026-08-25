// Package statistics stores bounded, content-free clipboard activity totals.
//
// It deliberately records time buckets rather than individual copy events. A
// user who copies ten times or ten million times therefore uses essentially the
// same amount of disk space, and no clipboard payload, hash, or source app is
// added to the privacy footprint.
package statistics

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"geda-clipboard/internal/appdir"
)

const (
	diskVersion     = 1
	retentionDays   = 370
	maxArchivedDays = retentionDays - 1 // today is held in the hourly bucket
	statisticsFile  = "statistics.json"
	saveDelay       = 750 * time.Millisecond
	KindText        = "text"
	KindImage       = "image"
	RangeDay        = "day"
	RangeWeek       = "week"
	RangeMonth      = "month"
	RangeYear       = "year"
)

// Counts is the content-free activity summary for one time bucket. Repeated is
// a subset of Total; Text and Image add up to Total.
type Counts struct {
	Total    int64 `json:"total"`
	Text     int64 `json:"text"`
	Image    int64 `json:"image"`
	Repeated int64 `json:"repeated"`
}

func (c *Counts) add(kind string, repeated bool) {
	c.Total++
	if kind == KindImage {
		c.Image++
	} else {
		c.Text++
	}
	if repeated {
		c.Repeated++
	}
}

func (c *Counts) addCounts(other Counts) {
	c.Total += other.Total
	c.Text += other.Text
	c.Image += other.Image
	c.Repeated += other.Repeated
}

type dayBucket struct {
	Date   string `json:"date"`
	Counts Counts `json:"counts"`
}

type diskData struct {
	Version     int         `json:"version"`
	StartedAt   *time.Time  `json:"startedAt,omitempty"`
	HourlyDate  string      `json:"hourlyDate"`
	Hourly      [24]Counts  `json:"hourly"`
	ArchivedDay []dayBucket `json:"days,omitempty"`
}

// Point is one plotted bucket. Start is a local timestamp so the frontend can
// format it in the same calendar the store used to group events.
type Point struct {
	Start  time.Time `json:"start"`
	Counts Counts    `json:"counts"`
}

// Snapshot is the chart-ready summary returned to the frontend.
type Snapshot struct {
	Period        string     `json:"period"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	RetentionDays int        `json:"retentionDays"`
	Totals        Counts     `json:"totals"`
	Points        []Point    `json:"points"`
}

// Store persists a fixed number of aggregate buckets. All methods are safe for
// concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
	data diskData

	saveMu      sync.Mutex
	pendingSave bool
	saveTimer   *time.Timer
}

// Open loads statistics from the user's application data directory.
func Open() (*Store, error) {
	dir, err := appdir.Data()
	if err != nil {
		return nil, err
	}
	return OpenAt(dir)
}

// OpenAt loads statistics from dir. Missing data starts empty; unreadable data
// is preserved before a new bounded store replaces it.
func OpenAt(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	s := &Store{path: filepath.Join(dir, statisticsFile)}
	s.data = emptyData(time.Now())

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return s, fmt.Errorf("read statistics: %w", err)
	}

	var stored diskData
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Version != diskVersion {
		preserveUnreadable(s.path)
		return s, nil
	}

	if stored.HourlyDate == "" {
		stored.HourlyDate = dateKey(time.Now())
	}
	s.data = stored
	s.advanceLocked(time.Now())
	s.pruneLocked(time.Now())
	return s, nil
}

func emptyData(now time.Time) diskData {
	return diskData{Version: diskVersion, HourlyDate: dateKey(now)}
}

func preserveUnreadable(path string) {
	dest := path + ".unreadable"
	if _, err := os.Stat(dest); err == nil {
		return
	}
	_ = os.Rename(path, dest)
}

func dateKey(t time.Time) string { return t.Format("2006-01-02") }

func parseDate(key string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", key, loc)
}

// Record adds one clipboard capture to its aggregate bucket. repeated means
// the history store matched the payload to an entry it already retained.
func (s *Store) Record(kind string, repeated bool) error {
	return s.RecordAt(kind, repeated, time.Now())
}

// RecordAt is Record with an explicit timestamp, primarily for deterministic
// tests.
func (s *Store) RecordAt(kind string, repeated bool, at time.Time) error {
	if kind != KindText && kind != KindImage {
		return fmt.Errorf("unknown statistics kind %q", kind)
	}
	if at.IsZero() {
		at = time.Now()
	}

	s.mu.Lock()
	if s.data.StartedAt == nil {
		started := at
		s.data.StartedAt = &started
	}
	currentDate := s.data.HourlyDate
	eventDate := dateKey(at)
	// A timezone change can move the local calendar backwards as well as
	// forwards. Runtime captures arrive in order, so any date change means the
	// old hourly bucket is complete and the new local day should own the event.
	if eventDate != currentDate {
		s.archiveHourlyLocked()
		s.data.HourlyDate = eventDate
		s.data.Hourly = [24]Counts{}
	}
	if eventDate == s.data.HourlyDate {
		s.data.Hourly[at.Hour()].add(kind, repeated)
	} else {
		s.addArchivedLocked(eventDate, kind, repeated)
	}
	s.pruneLocked(at)
	s.mu.Unlock()

	s.scheduleSave()
	return nil
}

func (s *Store) archiveHourlyLocked() {
	var total Counts
	for _, hour := range s.data.Hourly {
		total.addCounts(hour)
	}
	if total.Total == 0 || s.data.HourlyDate == "" {
		return
	}
	s.upsertDayLocked(s.data.HourlyDate, total)
}

func (s *Store) addArchivedLocked(date, kind string, repeated bool) {
	for i := range s.data.ArchivedDay {
		if s.data.ArchivedDay[i].Date == date {
			s.data.ArchivedDay[i].Counts.add(kind, repeated)
			return
		}
	}
	var counts Counts
	counts.add(kind, repeated)
	s.data.ArchivedDay = append(s.data.ArchivedDay, dayBucket{Date: date, Counts: counts})
}

func (s *Store) upsertDayLocked(date string, counts Counts) {
	for i := range s.data.ArchivedDay {
		if s.data.ArchivedDay[i].Date == date {
			s.data.ArchivedDay[i].Counts.addCounts(counts)
			return
		}
	}
	s.data.ArchivedDay = append(s.data.ArchivedDay, dayBucket{Date: date, Counts: counts})
}

// advanceLocked rolls the hourly bucket into one daily total when the local
// calendar advances. The bool reports whether persistent state changed.
func (s *Store) advanceLocked(now time.Time) bool {
	nowDate := dateKey(now)
	if nowDate == s.data.HourlyDate {
		return false
	}
	s.archiveHourlyLocked()
	s.data.HourlyDate = nowDate
	s.data.Hourly = [24]Counts{}
	return true
}

func (s *Store) pruneLocked(now time.Time) {
	cutoff := dateKey(startOfDay(now).AddDate(0, 0, -(retentionDays - 1)))
	kept := s.data.ArchivedDay[:0]
	for _, day := range s.data.ArchivedDay {
		if day.Date >= cutoff && day.Date != s.data.HourlyDate {
			kept = append(kept, day)
		}
	}
	s.data.ArchivedDay = kept
	sort.Slice(s.data.ArchivedDay, func(i, j int) bool {
		return s.data.ArchivedDay[i].Date < s.data.ArchivedDay[j].Date
	})
	if len(s.data.ArchivedDay) > maxArchivedDays {
		s.data.ArchivedDay = s.data.ArchivedDay[len(s.data.ArchivedDay)-maxArchivedDays:]
	}
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// Snapshot returns chart points for today, the last seven days, the last 30
// days, or the last 12 calendar months.
func (s *Store) Snapshot(period string) (Snapshot, error) {
	return s.SnapshotAt(period, time.Now())
}

// SnapshotAt is Snapshot with an explicit current time for tests.
func (s *Store) SnapshotAt(period string, now time.Time) (Snapshot, error) {
	if period != RangeDay && period != RangeWeek && period != RangeMonth && period != RangeYear {
		return Snapshot{}, fmt.Errorf("unknown statistics period %q", period)
	}

	s.mu.Lock()
	changed := s.advanceLocked(now)
	s.pruneLocked(now)
	startedAt := s.data.StartedAt
	if startedAt != nil {
		copyTime := *startedAt
		startedAt = &copyTime
	}
	hourlyDate := s.data.HourlyDate
	hourly := s.data.Hourly
	days := append([]dayBucket(nil), s.data.ArchivedDay...)
	s.mu.Unlock()
	if changed {
		s.scheduleSave()
	}

	snapshot := Snapshot{
		Period:        period,
		StartedAt:     startedAt,
		RetentionDays: retentionDays,
		Points:        make([]Point, 0, 24),
	}

	dayCounts := make(map[string]Counts, len(days)+1)
	for _, day := range days {
		dayCounts[day.Date] = day.Counts
	}
	var todayCounts Counts
	for _, hour := range hourly {
		todayCounts.addCounts(hour)
	}
	dayCounts[hourlyDate] = todayCounts

	switch period {
	case RangeDay:
		dayStart := startOfDay(now)
		for hour := 0; hour < 24; hour++ {
			counts := Counts{}
			if dateKey(now) == hourlyDate {
				counts = hourly[hour]
			}
			snapshot.Points = append(snapshot.Points, Point{
				Start:  dayStart.Add(time.Duration(hour) * time.Hour),
				Counts: counts,
			})
		}
	case RangeWeek:
		snapshot.Points = dailyPoints(dayCounts, now, 7)
	case RangeMonth:
		snapshot.Points = dailyPoints(dayCounts, now, 30)
	case RangeYear:
		snapshot.Points = monthlyPoints(dayCounts, now)
	}

	for _, point := range snapshot.Points {
		snapshot.Totals.addCounts(point.Counts)
	}
	return snapshot, nil
}

func dailyPoints(counts map[string]Counts, now time.Time, days int) []Point {
	points := make([]Point, 0, days)
	first := startOfDay(now).AddDate(0, 0, -(days - 1))
	for offset := 0; offset < days; offset++ {
		start := first.AddDate(0, 0, offset)
		points = append(points, Point{Start: start, Counts: counts[dateKey(start)]})
	}
	return points
}

func monthlyPoints(counts map[string]Counts, now time.Time) []Point {
	loc := now.Location()
	first := time.Date(now.Year(), now.Month()-11, 1, 0, 0, 0, 0, loc)
	points := make([]Point, 12)
	for i := range points {
		points[i].Start = first.AddDate(0, i, 0)
	}
	for date, count := range counts {
		day, err := parseDate(date, loc)
		if err != nil || day.Before(first) {
			continue
		}
		monthOffset := (day.Year()-first.Year())*12 + int(day.Month()-first.Month())
		if monthOffset >= 0 && monthOffset < len(points) {
			points[monthOffset].Counts.addCounts(count)
		}
	}
	return points
}

// Reset removes all aggregate statistics while leaving clipboard history and
// preferences untouched.
func (s *Store) Reset() error {
	s.mu.Lock()
	s.data = emptyData(time.Now())
	s.mu.Unlock()
	return s.Flush()
}

func (s *Store) scheduleSave() {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.pendingSave = true
	s.saveTimer = time.AfterFunc(saveDelay, func() {
		if err := s.Flush(); err != nil {
			fmt.Fprintln(os.Stderr, "geda-clipboard: save statistics:", err)
		}
	})
}

// Flush writes the bounded aggregate file immediately.
func (s *Store) Flush() error {
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	// Keep snapshotting and the atomic rename in one serial section. Otherwise
	// an older timer flush can finish after Reset and restore stale counters.
	s.pendingSave = false

	s.mu.Lock()
	data, err := json.Marshal(s.data)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return appdir.WriteAtomic(s.path, data, 0o600)
}

// Close stops the save timer and persists pending counts.
func (s *Store) Close() error {
	s.saveMu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	pending := s.pendingSave
	s.saveMu.Unlock()
	if !pending {
		return nil
	}
	return s.Flush()
}
