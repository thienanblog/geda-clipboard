// Command asc asks App Store Connect what it already knows about this app, and
// refuses a package that would be rejected on upload.
//
// Two rules, and the second one is the reason this exists at all:
//
//   - A CFBundleVersion is single use for the app and must be higher than every
//     previous upload, including builds uploaded under an older marketing
//     version. App Store Connect reports this only after the upload.
//
//   - A marketing version is single use too, and far less forgiving. Once
//     CFBundleShortVersionString has been released it can never be submitted
//     again: raising only the build number does not reopen it. The fix is to
//     create a new version in App Store Connect and bump the app to match,
//     which means editing wails.json and main.go and cutting a release. Finding
//     that out from a rejection notice costs a day; finding it out here costs a
//     second.
//
// Credentials come from the environment, and their absence is not an error:
// without them the check is skipped with a warning, the same way signing is
// opt-in elsewhere in this repository, so a fork can still package.
//
//	AC_API_KEY_P8         path to the .p8, or
//	AC_API_KEY_P8_BASE64  its base64 contents, as CI holds it
//	AC_API_KEY_ID         the key ID, the XXXXXXXXXX in AuthKey_XXXXXXXXXX.p8
//	AC_API_ISSUER_ID      the issuer UUID from Users and Access, Integrations
//
// Usage:
//
//	go run ./scripts/asc preflight -bundle-id com.geda.clipboard -version 0.6.0 -build 3
package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.appstoreconnect.apple.com"

// States a version can be in after it has gone live. Reaching any of these
// burns the marketing version permanently: App Store Connect will not accept
// another build for it, whatever the build number says.
//
// REPLACED_WITH_NEW_VERSION is included because it means exactly the same
// thing from here -- the version was released and something later superseded
// it. Both readings of "already released" land in this set.
var releasedStates = map[string]bool{
	"READY_FOR_SALE":              true,
	"REPLACED_WITH_NEW_VERSION":   true,
	"PENDING_DEVELOPER_RELEASE":   true,
	"PROCESSING_FOR_DISTRIBUTION": true,
}

func main() {
	if len(os.Args) < 2 || os.Args[1] != "preflight" {
		fmt.Fprintln(os.Stderr, "usage: asc preflight -bundle-id ID -version X.Y.Z -build N")
		os.Exit(2)
	}

	fs := flag.NewFlagSet("preflight", flag.ExitOnError)
	bundleID := fs.String("bundle-id", "", "the app's bundle identifier")
	version := fs.String("version", "", "CFBundleShortVersionString about to be packaged")
	build := fs.String("build", "", "CFBundleVersion about to be packaged")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *bundleID == "" || *version == "" || *build == "" {
		fmt.Fprintln(os.Stderr, "bundle-id, version and build are all required")
		os.Exit(2)
	}

	if err := preflight(*bundleID, *version, *build); err != nil {
		fmt.Fprintf(os.Stderr, "\n%v\n", err)
		os.Exit(1)
	}
}

func preflight(bundleID, version, build string) error {
	client, err := newClient()
	if err != nil {
		if errors.Is(err, errNoCredentials) {
			fmt.Println("    App Store Connect credentials are not set; skipping the version check.")
			fmt.Println("    Set AC_API_KEY_P8, AC_API_KEY_ID and AC_API_ISSUER_ID to enable it.")
			return nil
		}
		return err
	}

	appID, err := client.appID(bundleID)
	if err != nil {
		return err
	}
	if appID == "" {
		fmt.Printf("    No app with bundle ID %s in App Store Connect; treating this as the first submission.\n", bundleID)
		return nil
	}

	versions, err := client.versions(appID)
	if err != nil {
		return err
	}

	for _, v := range versions {
		if v.Version != version {
			continue
		}
		if releasedStates[v.State] {
			return fmt.Errorf(`version %s has already been released (App Store Connect reports %s).

A released marketing version cannot be submitted again, and raising only the
build number will not reopen it. To ship a change:

  1. Create a new version in App Store Connect, higher than %s.
  2. Bump appVersion in main.go and info.productVersion in wails.json to match.
  3. Move the Unreleased entries in CHANGELOG.md under it, tag, and release.
  4. Package again with BUILD_NUMBER starting from 1 for that new version.`,
				version, v.State, version)
		}
		fmt.Printf("    Version %s exists and is editable (%s).\n", version, v.State)
	}

	// A version lower than one already on sale is refused for the same reason,
	// and reads as a much more confusing error when Apple reports it.
	for _, v := range versions {
		if !releasedStates[v.State] {
			continue
		}
		if compareVersions(version, v.Version) < 0 {
			return fmt.Errorf("version %s is lower than %s, which is already released; "+
				"the App Store only accepts an increasing CFBundleShortVersionString", version, v.Version)
		}
	}

	builds, err := client.builds(appID)
	if err != nil {
		return err
	}
	highest := ""
	for _, b := range builds {
		if b == build {
			return fmt.Errorf("build %s has already been uploaded; "+
				"App Store Connect rejects a CFBundleVersion it has seen before, so set BUILD_NUMBER higher", build)
		}
		if highest == "" || compareVersions(b, highest) > 0 {
			highest = b
		}
	}
	if highest != "" && compareVersions(build, highest) <= 0 {
		return fmt.Errorf("build %s is not higher than %s, the highest build already uploaded for this app; "+
			"set BUILD_NUMBER to %s or more", build, highest, nextAfter(highest))
	}

	if highest == "" {
		fmt.Printf("    No builds uploaded yet for this app; %s is free.\n", build)
	} else {
		fmt.Printf("    Highest build uploaded for this app is %s; %s is free.\n", highest, build)
	}
	return nil
}

// ---------------------------------------------------------------------------
// App Store Connect client
// ---------------------------------------------------------------------------

var errNoCredentials = errors.New("no App Store Connect credentials in the environment")

type client struct {
	token   string
	http    *http.Client
	baseURL string
}

func newClient() (*client, error) {
	keyID := os.Getenv("AC_API_KEY_ID")
	issuer := os.Getenv("AC_API_ISSUER_ID")
	if keyID == "" || issuer == "" {
		return nil, errNoCredentials
	}

	var raw []byte
	switch {
	case os.Getenv("AC_API_KEY_P8_BASE64") != "":
		decoded, err := base64.StdEncoding.DecodeString(os.Getenv("AC_API_KEY_P8_BASE64"))
		if err != nil {
			return nil, fmt.Errorf("AC_API_KEY_P8_BASE64 is not valid base64: %w", err)
		}
		raw = decoded
	case os.Getenv("AC_API_KEY_P8") != "":
		b, err := os.ReadFile(os.Getenv("AC_API_KEY_P8"))
		if err != nil {
			return nil, fmt.Errorf("read AC_API_KEY_P8: %w", err)
		}
		raw = b
	default:
		return nil, errNoCredentials
	}

	token, err := signJWT(raw, keyID, issuer)
	if err != nil {
		return nil, err
	}
	return &client{
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: apiBase,
	}, nil
}

// signJWT builds the ES256 token App Store Connect authenticates with.
//
// Done here rather than with a library because the Go standard library already
// has every piece, and a release tool that pulls a dependency into go.mod for
// forty lines of work is a dependency the app itself then carries.
func signJWT(p8 []byte, keyID, issuer string) (string, error) {
	block, _ := pem.Decode(p8)
	if block == nil {
		return "", errors.New("the API key is not PEM; it should start with BEGIN PRIVATE KEY")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse the API key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("the API key is %T, not the ECDSA key App Store Connect issues", parsed)
	}

	now := time.Now()
	header := map[string]any{"alg": "ES256", "kid": keyID, "typ": "JWT"}
	// Twenty minutes is Apple's ceiling; a token minted past it is refused
	// outright rather than merely expiring early.
	payload := map[string]any{
		"iss": issuer,
		"iat": now.Unix(),
		"exp": now.Add(20 * time.Minute).Unix(),
		"aud": "appstoreconnect-v1",
	}

	segment := func(v any) (string, error) {
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return base64.RawURLEncoding.EncodeToString(b), nil
	}
	h, err := segment(header)
	if err != nil {
		return "", err
	}
	p, err := segment(payload)
	if err != nil {
		return "", err
	}

	input := h + "." + p
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	// JWS wants the raw r||s pair, each padded to the curve's 32 bytes. Go's
	// ecdsa.SignASN1 returns DER instead, which App Store Connect rejects
	// without explaining why, so the two halves are laid out by hand.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])

	return input + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (c *client) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("App Store Connect request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("App Store Connect rejected the key (401). Check AC_API_KEY_ID and "+
			"AC_API_ISSUER_ID against Users and Access, Integrations.\n%s", strings.TrimSpace(string(body)))
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("App Store Connect returned %s for %s\n%s",
			resp.Status, path, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, out)
}

func (c *client) appID(bundleID string) (string, error) {
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	path := "/v1/apps?filter[bundleId]=" + url.QueryEscape(bundleID)
	if err := c.get(path, &out); err != nil {
		return "", err
	}
	if len(out.Data) == 0 {
		return "", nil
	}
	return out.Data[0].ID, nil
}

type versionInfo struct {
	Version string
	State   string
}

func (c *client) versions(appID string) ([]versionInfo, error) {
	var out struct {
		Data []struct {
			Attributes struct {
				VersionString string `json:"versionString"`
				// appStoreState is the long-standing field; appVersionState is
				// what newer API versions return. Whichever is populated wins,
				// so this keeps working across the change rather than silently
				// deciding every version is editable.
				AppStoreState   string `json:"appStoreState"`
				AppVersionState string `json:"appVersionState"`
			} `json:"attributes"`
		} `json:"data"`
	}
	path := "/v1/apps/" + url.PathEscape(appID) +
		"/appStoreVersions?filter[platform]=MAC_OS&limit=200"
	if err := c.get(path, &out); err != nil {
		return nil, err
	}

	versions := make([]versionInfo, 0, len(out.Data))
	for _, d := range out.Data {
		state := d.Attributes.AppStoreState
		if state == "" {
			state = d.Attributes.AppVersionState
		}
		versions = append(versions, versionInfo{
			Version: d.Attributes.VersionString,
			State:   state,
		})
	}
	return versions, nil
}

// builds returns every CFBundleVersion already uploaded for the app. Apple's
// ordering requirement spans marketing versions, so filtering by
// preReleaseVersion.version would miss an older build that still sets the
// minimum accepted number.
func (c *client) builds(appID string) ([]string, error) {
	var out struct {
		Data []struct {
			Attributes struct {
				Version string `json:"version"`
			} `json:"attributes"`
		} `json:"data"`
	}
	path := "/v1/builds?filter[app]=" + url.QueryEscape(appID) + "&limit=200"
	if err := c.get(path, &out); err != nil {
		return nil, err
	}

	builds := make([]string, 0, len(out.Data))
	for _, d := range out.Data {
		if d.Attributes.Version != "" {
			builds = append(builds, d.Attributes.Version)
		}
	}
	return builds, nil
}

// ---------------------------------------------------------------------------
// Version arithmetic
// ---------------------------------------------------------------------------

// compareVersions orders dotted numeric versions: -1 if a sorts before b, 1 if
// after, 0 if equal. Both CFBundleVersion and CFBundleShortVersionString are
// dot-separated integers, so this covers "3" against "10" as well as "0.9.0"
// against "0.10.0", neither of which a string comparison gets right.
//
// A component that is not a number sorts as 0 rather than failing. Apple
// accepts only digits and dots in either field, so reaching that branch means
// the input was already invalid and a confusing ordering is not the problem.
func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		av, bv := 0, 0
		if i < len(as) {
			av, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bv, _ = strconv.Atoi(bs[i])
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

// nextAfter suggests the smallest build number that would be accepted, by
// bumping the last component.
func nextAfter(v string) string {
	parts := strings.Split(v, ".")
	last := parts[len(parts)-1]
	n, err := strconv.Atoi(last)
	if err != nil {
		return v + ".1"
	}
	parts[len(parts)-1] = strconv.Itoa(n + 1)
	return strings.Join(parts, ".")
}
