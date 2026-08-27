package settings

import (
	"os"
	"path/filepath"
	"testing"

	"geda-clipboard/internal/appdir"
)

func TestNormaliseClampsBothEnds(t *testing.T) {
	d := Defaults()

	cases := []struct {
		name string
		in   Settings
		want Settings
	}{
		{
			name: "below the minimum",
			in:   Settings{MaxItems: 1, PopupWidth: 10, PopupHeight: 10},
			want: Settings{MaxItems: 10, PopupWidth: d.PopupWidth, PopupHeight: d.PopupHeight},
		},
		{
			// A number input does not enforce its max for a typed value, so an
			// absurd size reaches us and would produce a window that cannot be
			// dismissed from its own footer.
			name: "above the maximum",
			in:   Settings{MaxItems: 999999, PopupWidth: 99999, PopupHeight: 99999},
			want: Settings{MaxItems: 2000, PopupWidth: 1600, PopupHeight: 1200},
		},
		{
			name: "already in range",
			in:   Settings{MaxItems: 50, PopupWidth: 800, PopupHeight: 600},
			want: Settings{MaxItems: 50, PopupWidth: 800, PopupHeight: 600},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in
			got.normalise()

			if got.MaxItems != tc.want.MaxItems {
				t.Errorf("MaxItems = %d, want %d", got.MaxItems, tc.want.MaxItems)
			}
			if got.PopupWidth != tc.want.PopupWidth {
				t.Errorf("PopupWidth = %d, want %d", got.PopupWidth, tc.want.PopupWidth)
			}
			if got.PopupHeight != tc.want.PopupHeight {
				t.Errorf("PopupHeight = %d, want %d", got.PopupHeight, tc.want.PopupHeight)
			}
		})
	}
}

// Load must hand back a usable Manager even when it fails, because the caller
// only logs the error and then immediately reads the settings.
func TestLoadAlwaysReturnsAUsableManager(t *testing.T) {
	// An unset HOME makes os.UserConfigDir fail, which is the path that used to
	// return a nil Manager.
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")

	m, err := Load()
	if m == nil {
		t.Fatalf("Load returned a nil Manager (err = %v); callers dereference it", err)
	}

	if got := m.Get(); got.MaxItems != Defaults().MaxItems {
		t.Errorf("MaxItems = %d, want the default %d", got.MaxItems, Defaults().MaxItems)
	}
	if !m.NeedsWelcome() {
		t.Error("installation with no data directory should still show Welcome in memory")
	}

	if err == nil {
		t.Skip("this platform resolved a config directory anyway")
	}
	// With nowhere to write, Save must say so rather than dropping a file into
	// the working directory.
	if _, err := m.Save(Defaults()); err == nil {
		t.Error("Save with no data directory should return an error")
	}
	if err := m.CompleteWelcome(); err == nil {
		t.Error("completing Welcome with no data directory should return an error")
	}
	if m.NeedsWelcome() {
		t.Error("failed persistence should not trap this session on Welcome")
	}
}

func TestIsIgnoredIsCaseInsensitive(t *testing.T) {
	s := Settings{IgnoredApps: []string{"1Password", "Keychain Access"}}

	for _, name := range []string{"1password", "1PASSWORD", "Keychain Access"} {
		if !s.IsIgnored(name) {
			t.Errorf("IsIgnored(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "Safari", "1Password7"} {
		if s.IsIgnored(name) {
			t.Errorf("IsIgnored(%q) = true, want false", name)
		}
	}
}

func TestNormalisePopupPlacement(t *testing.T) {
	d := Defaults()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "cursor is kept", in: PlacementCursor, want: PlacementCursor},
		{name: "menu bar is kept", in: PlacementMenuBar, want: PlacementMenuBar},
		// A settings file written before the preference existed unmarshals over
		// the defaults, so this only happens for a hand-edited or truncated one.
		{name: "empty falls back", in: "", want: d.PopupPlacement},
		{name: "unknown falls back", in: "somewhere-else", want: d.PopupPlacement},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Settings{PopupPlacement: tc.in}
			got.normalise()

			if got.PopupPlacement != tc.want {
				t.Errorf("PopupPlacement = %q, want %q", got.PopupPlacement, tc.want)
			}
		})
	}
}

func TestNormaliseImagePreviewSize(t *testing.T) {
	for _, size := range []string{PreviewCompact, PreviewComfortable, PreviewLarge} {
		got := Defaults()
		got.ImagePreviewSize = size
		got.normalise()
		if got.ImagePreviewSize != size {
			t.Errorf("ImagePreviewSize = %q, want %q", got.ImagePreviewSize, size)
		}
	}

	got := Defaults()
	got.ImagePreviewSize = "enormous"
	got.normalise()
	if got.ImagePreviewSize != PreviewComfortable {
		t.Errorf("unknown ImagePreviewSize = %q, want %q", got.ImagePreviewSize, PreviewComfortable)
	}
}

// writeSettingsFile points the app data directory at a temporary HOME and puts
// the given JSON there as the stored configuration.
func writeSettingsFile(t *testing.T, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	dir, err := appdir.Data()
	if err != nil {
		t.Fatalf("data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func useEmptySettingsDirectory(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
}

func TestFreshInstallNeedsWelcomeUntilCompletionIsPersisted(t *testing.T) {
	useEmptySettingsDirectory(t)

	m, err := Load()
	if err != nil {
		t.Fatalf("load fresh settings: %v", err)
	}
	if !m.NeedsWelcome() {
		t.Fatal("fresh install does not need Welcome")
	}
	if err := m.CompleteWelcome(); err != nil {
		t.Fatalf("complete Welcome: %v", err)
	}
	if m.NeedsWelcome() {
		t.Fatal("completed Welcome still marked as needed")
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("reload settings: %v", err)
	}
	if reloaded.NeedsWelcome() {
		t.Fatal("Welcome completion was not persisted")
	}
}

func TestStoredSettingsSkipWelcome(t *testing.T) {
	writeSettingsFile(t, `{"popupWidth":420,"popupHeight":520,"layoutVersion":1}`)

	m, err := Load()
	if err != nil {
		t.Fatalf("load stored settings: %v", err)
	}
	if m.NeedsWelcome() {
		t.Fatal("existing settings unexpectedly need Welcome")
	}
}

func TestLegacySettingsUseSafePinnedClearDefault(t *testing.T) {
	writeSettingsFile(t, `{"popupWidth":420,"popupHeight":520,"layoutVersion":1}`)
	m, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got := m.Get()
	if got.ClearPinnedOnHistoryClear {
		t.Error("legacy settings opted pinned entries into history clearing")
	}
	if got.ImagePreviewSize != PreviewComfortable {
		t.Errorf("legacy preview size = %q, want %q", got.ImagePreviewSize, PreviewComfortable)
	}
}

// A configuration written before the detail card moved out of the panel was
// sized for the docked column, so its width is reset once.
func TestLoadMigratesWidthFromTheDockedLayout(t *testing.T) {
	writeSettingsFile(t, `{"popupWidth":720,"popupHeight":520}`)

	m, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := m.Get().PopupWidth; got != Defaults().PopupWidth {
		t.Errorf("popup width = %d, want %d", got, Defaults().PopupWidth)
	}
	if got := m.Get().PopupHeight; got != 520 {
		t.Errorf("popup height = %d, want it untouched at 520", got)
	}
}

// A width chosen under the current layout is the user's, however wide.
func TestLoadKeepsWidthChosenUnderTheCurrentLayout(t *testing.T) {
	writeSettingsFile(t, `{"popupWidth":900,"layoutVersion":1}`)

	m, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := m.Get().PopupWidth; got != 900 {
		t.Errorf("popup width = %d, want 900", got)
	}
}
