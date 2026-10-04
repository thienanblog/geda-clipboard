package main

import (
	"path/filepath"
	"testing"

	"geda-clipboard/internal/clipboard"
	"geda-clipboard/internal/settings"
	"geda-clipboard/internal/store"
)

func TestNotificationBodyRespectsPreviewPreference(t *testing.T) {
	for _, tc := range []struct {
		name string
		item store.Item
		want string
	}{
		{"text", store.Item{Kind: store.KindText, Text: "private\nmessage"}, "private message"},
		{"image", store.Item{Kind: store.KindImage, ImageW: 640, ImageH: 480}, "Image · 640 × 480"},
		{"file", store.Item{Kind: store.KindFile, Files: []store.FileReference{{Path: "/private/report.pdf", Name: "report.pdf"}}}, "report.pdf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := notificationBody(&tc.item, false); got != "Clipboard content hidden" {
				t.Fatalf("hidden preview leaked details: %q", got)
			}
			if got := notificationBody(&tc.item, true); got != tc.want {
				t.Fatalf("visible preview = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFileCapturePreferenceKeepsHistoryAndText(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("AppData", filepath.Join(dir, "AppData"))
	manager, err := settings.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg := manager.Get()
	cfg.NotifyOnCopy = false
	if _, err := manager.Save(cfg); err != nil {
		t.Fatal(err)
	}
	history, err := store.OpenAt(t.TempDir(), 200)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { history.Close() })
	app := &App{settings: manager, store: history}
	source := clipboard.App{Name: "Fixture"}
	file := clipboard.Snapshot{Kind: clipboard.KindFile, Files: []clipboard.FileReference{{Path: filepath.Join(dir, "first.txt")}}}
	app.onClipboardChange(file, source)
	if history.Count() != 1 {
		t.Fatal("file capture should be enabled by default")
	}
	cfg.CaptureFiles = false
	if _, err := manager.Save(cfg); err != nil {
		t.Fatal(err)
	}
	file.Files[0].Path = filepath.Join(dir, "second.txt")
	app.onClipboardChange(file, source)
	if history.Count() != 1 {
		t.Fatal("disabling file capture must skip new files and retain existing history")
	}
	app.onClipboardChange(clipboard.Snapshot{Kind: clipboard.KindText, Text: "Still recorded"}, source)
	if history.Count() != 2 {
		t.Fatal("disabling file capture must not disable text capture")
	}
	cfg.CaptureFiles = true
	if _, err := manager.Save(cfg); err != nil {
		t.Fatal(err)
	}
	app.onClipboardChange(file, source)
	if history.Count() != 3 {
		t.Fatal("re-enabling file capture should record new files immediately")
	}
}
