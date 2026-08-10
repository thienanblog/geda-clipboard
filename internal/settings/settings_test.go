package settings

import "testing"

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

	if err == nil {
		t.Skip("this platform resolved a config directory anyway")
	}
	// With nowhere to write, Save must say so rather than dropping a file into
	// the working directory.
	if _, err := m.Save(Defaults()); err == nil {
		t.Error("Save with no data directory should return an error")
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
