package statistics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func localTime(year int, month time.Month, day, hour int) time.Time {
	return time.Date(year, month, day, hour, 0, 0, 0, time.Local)
}

func TestSnapshotCountsKindsAndRepeatedCopies(t *testing.T) {
	s, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := localTime(2026, time.August, 26, 15)
	for _, event := range []struct {
		kind     string
		repeated bool
		hour     int
	}{
		{KindText, false, 9},
		{KindText, true, 9},
		{KindImage, false, 15},
	} {
		if err := s.RecordAt(event.kind, event.repeated, localTime(2026, time.August, 26, event.hour)); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.SnapshotAt(RangeDay, now)
	if err != nil {
		t.Fatal(err)
	}
	want := (Counts{Total: 3, Text: 2, Image: 1, Repeated: 1})
	if got.Totals != want {
		t.Fatalf("Totals = %+v, want %+v", got.Totals, want)
	}
	if got.Points[9].Counts.Total != 2 || got.Points[15].Counts.Image != 1 {
		t.Fatalf("hourly points = %+v", got.Points)
	}
}

func TestRollingRangesArchiveWithoutPerEventGrowth(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(dir)
	if err != nil {
		t.Fatal(err)
	}

	start := localTime(2025, time.August, 23, 10)
	for day := 0; day < 380; day++ {
		at := start.AddDate(0, 0, day)
		if err := s.RecordAt(KindText, day%2 == 0, at); err != nil {
			t.Fatal(err)
		}
	}
	now := start.AddDate(0, 0, 379)
	year, err := s.SnapshotAt(RangeYear, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(year.Points) != 12 {
		t.Fatalf("year points = %d, want 12", len(year.Points))
	}
	firstMonth := time.Date(now.Year(), now.Month()-11, 1, 0, 0, 0, 0, now.Location())
	var wantYear int64
	for day := 0; day < 380; day++ {
		if at := start.AddDate(0, 0, day); !at.Before(firstMonth) && !at.After(now) {
			wantYear++
		}
	}
	if year.Totals.Total != wantYear {
		t.Fatalf("year total = %d, want %d", year.Totals.Total, wantYear)
	}
	if got := len(s.data.ArchivedDay); got != maxArchivedDays {
		t.Fatalf("archived days = %d, want bounded %d", got, maxArchivedDays)
	}

	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, statisticsFile))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 50_000 {
		t.Fatalf("bounded statistics file is %d bytes, want <= 50000", len(raw))
	}
}

func TestCopyVolumeChangesCountersNotFileShape(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	at := localTime(2026, time.August, 26, 10)
	if err := s.RecordAt(KindText, false, at); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	one, err := os.Stat(filepath.Join(dir, statisticsFile))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10_000; i++ {
		if err := s.RecordAt(KindText, true, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	many, err := os.Stat(filepath.Join(dir, statisticsFile))
	if err != nil {
		t.Fatal(err)
	}
	if growth := many.Size() - one.Size(); growth > 32 {
		t.Fatalf("10000 copies grew file by %d bytes; want counter digits only", growth)
	}
}

func TestResetRemovesStatistics(t *testing.T) {
	s, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAt(KindImage, true, localTime(2026, time.August, 26, 10)); err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	got, err := s.SnapshotAt(RangeDay, localTime(2026, time.August, 26, 11))
	if err != nil {
		t.Fatal(err)
	}
	if got.Totals.Total != 0 || got.StartedAt != nil {
		t.Fatalf("snapshot after reset = %+v", got)
	}
}

func TestFlushAndReopenPreservesCounters(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	s, err := OpenAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAt(KindText, false, now); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAt(KindImage, true, now); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.SnapshotAt(RangeDay, now)
	if err != nil {
		t.Fatal(err)
	}
	want := (Counts{Total: 2, Text: 1, Image: 1, Repeated: 1})
	if got.Totals != want {
		t.Fatalf("reopened totals = %+v, want %+v", got.Totals, want)
	}
}

func TestTimezoneChangeCanMoveHourlyDateBackwards(t *testing.T) {
	s, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	ahead := time.FixedZone("ahead", 14*60*60)
	behind := time.FixedZone("behind", -10*60*60)
	s.data = emptyData(time.Date(2026, time.August, 27, 0, 30, 0, 0, ahead))
	now := time.Date(2026, time.August, 26, 23, 30, 0, 0, behind)
	if err := s.RecordAt(KindText, false, now); err != nil {
		t.Fatal(err)
	}

	got, err := s.SnapshotAt(RangeDay, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Totals.Text != 1 || s.data.HourlyDate != "2026-08-26" {
		t.Fatalf("backward date rollover = date %s totals %+v", s.data.HourlyDate, got.Totals)
	}
}
