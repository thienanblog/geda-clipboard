package clipboard

import (
	"errors"
	"testing"
)

// fakeClipboard drives a Watcher without touching the OS: tests set the counter
// and what a read returns, then call tick() to advance one poll.
type fakeClipboard struct {
	count int64
	snap  Snapshot
	err   error

	reads int

	got []Snapshot
}

func newFakeWatcher() (*Watcher, *fakeClipboard) {
	fake := &fakeClipboard{}

	w := NewWatcher(nil)
	w.counter = func() int64 { return fake.count }
	w.reader = func() (Snapshot, error) {
		fake.reads++
		return fake.snap, fake.err
	}
	w.source = func() App { return App{Name: "TextEdit", BundleID: "com.apple.TextEdit"} }
	w.OnChange = func(s Snapshot, _ App) { fake.got = append(fake.got, s) }

	w.last = fake.count
	return w, fake
}

func text(s string) Snapshot { return Snapshot{Kind: KindText, Text: s} }

// pendingRemote is what the pasteboard looks like between a Universal
// Clipboard copy being announced and its payload arriving: the marker is
// there, the content is not.
func pendingRemote() Snapshot { return Snapshot{Remote: true} }

func remoteText(s string) Snapshot {
	snap := text(s)
	snap.Remote = true
	return snap
}

func TestTickReportsCopy(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count, fake.snap = 1, text("hello")
	w.tick()

	if len(fake.got) != 1 {
		t.Fatalf("got %d copies, want 1", len(fake.got))
	}
	if fake.got[0].Text != "hello" {
		t.Errorf("Text = %q, want %q", fake.got[0].Text, "hello")
	}
	if fake.got[0].Change != 1 {
		t.Errorf("Change = %d, want 1", fake.got[0].Change)
	}
}

func TestTickIgnoresOwnWrite(t *testing.T) {
	w, fake := newFakeWatcher()

	w.Ignore(1)
	fake.count, fake.snap = 1, text("pasted back")
	w.tick()

	if len(fake.got) != 0 {
		t.Fatalf("own write was reported as a copy: %+v", fake.got)
	}
	if fake.reads != 0 {
		t.Errorf("clipboard was read %d times for an ignored change, want 0", fake.reads)
	}
}

func TestTickWithoutChangeDoesNotRead(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count, fake.snap = 1, text("hello")
	w.tick()
	w.tick()
	w.tick()

	if len(fake.got) != 1 {
		t.Fatalf("got %d copies, want 1", len(fake.got))
	}
	if fake.reads != 1 {
		t.Errorf("clipboard was read %d times, want 1", fake.reads)
	}
}

// The payload of a Universal Clipboard copy arrives after the change counter
// moves, so an empty first read has to be retried rather than dropped.
func TestTickRetriesUntilPayloadArrives(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count = 7
	fake.snap = pendingRemote() // announced, payload not yet transferred
	w.tick()
	w.tick()

	if len(fake.got) != 0 {
		t.Fatalf("reported a copy with no payload: %+v", fake.got)
	}

	fake.snap = remoteText("from the phone")
	w.tick()

	if len(fake.got) != 1 {
		t.Fatalf("got %d copies, want 1", len(fake.got))
	}
	if fake.got[0].Change != 7 {
		t.Errorf("Change = %d, want 7 (the counter that announced it)", fake.got[0].Change)
	}
}

// A local app can publish readable fallback text while its preferred lazy image
// is still rendering. Pending must win, or the fallback consumes the change and
// the image is lost when it arrives without another counter increment.
func TestTickRetriesAdvertisedLocalImageUntilDataArrives(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count = 7
	fake.snap = Snapshot{Kind: KindText, Text: "fallback", Pending: true}
	w.tick()
	w.tick()

	if len(fake.got) != 0 {
		t.Fatalf("reported the fallback before the image was ready: %+v", fake.got)
	}

	fake.snap = Snapshot{Kind: KindImage, Image: []byte("png")}
	w.tick()

	if len(fake.got) != 1 {
		t.Fatalf("got %d copies, want 1", len(fake.got))
	}
	if fake.got[0].Change != 7 || fake.got[0].Kind != KindImage {
		t.Errorf("reported snapshot = %+v, want image for change 7", fake.got[0])
	}
}

func TestTickGivesUpAfterPendingReadTicks(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count = 7
	fake.snap = pendingRemote()
	for i := 0; i < pendingReadTicks+5; i++ {
		w.tick()
	}
	if fake.reads != pendingReadTicks {
		t.Fatalf("clipboard was read %d times, want %d", fake.reads, pendingReadTicks)
	}

	// The payload arriving after the watcher stopped waiting is not reported:
	// the change it belonged to is over.
	fake.snap = remoteText("too late")
	w.tick()
	if len(fake.got) != 0 {
		t.Errorf("reported a copy after giving up: %+v", fake.got)
	}
}

func TestTickAbandonsPendingChangeWhenANewerCopyArrives(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count, fake.snap = 7, pendingRemote()
	w.tick()

	fake.count, fake.snap = 8, text("newer")
	w.tick()

	if len(fake.got) != 1 {
		t.Fatalf("got %d copies, want 1", len(fake.got))
	}
	if fake.got[0].Change != 8 {
		t.Errorf("Change = %d, want 8", fake.got[0].Change)
	}
}

// Only a payload that may still be crossing from another device is worth
// waiting for. A local copy of something this app does not record -- a file
// copied in Finder -- must not keep the clipboard busy for three seconds.
func TestTickDoesNotRetryAnUnreadableLocalCopy(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count = 7
	fake.snap = Snapshot{} // KindNone, no remote marker
	w.tick()
	w.tick()
	w.tick()

	if fake.reads != 1 {
		t.Errorf("clipboard was read %d times, want 1", fake.reads)
	}
	if len(fake.got) != 0 {
		t.Errorf("reported a copy with no payload: %+v", fake.got)
	}
}

// A read that fails outright says nothing about the payload, so it is retried.
func TestTickRetriesAfterAReadError(t *testing.T) {
	w, fake := newFakeWatcher()

	fake.count = 7
	fake.err = errors.New("clipboard busy")
	w.tick()
	w.tick()

	fake.err, fake.snap = nil, text("readable now")
	w.tick()

	if len(fake.got) != 1 {
		t.Fatalf("got %d copies, want 1", len(fake.got))
	}
}

// Nothing is deduplicated by content, remote copies included: macOS bumps the
// change counter once per Universal Clipboard copy, and a repeat copy of
// identical content is a copy, which is what the copy counter is for.
func TestRepeatedPayloadIsReportedEachTime(t *testing.T) {
	for _, tc := range []struct {
		name string
		snap Snapshot
	}{
		{"local", text("same text")},
		{"remote", remoteText("same text")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, fake := newFakeWatcher()

			fake.count, fake.snap = 1, tc.snap
			w.tick()
			fake.count = 2
			w.tick()

			if len(fake.got) != 2 {
				t.Fatalf("got %d copies, want 2", len(fake.got))
			}
		})
	}
}
