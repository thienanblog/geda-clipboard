# App Store Connect submission sheet

Everything App Store Connect asks for, written out so a submission is a matter
of copying fields across rather than re-inventing the wording each time.

Kept in the repository, next to the entitlements and the packaging script, so
the text that ships and the build that ships move together. Character counts
below are Apple's limits; `scripts/asc/` enforces the version rules, and
`go test ./...` enforces the bundle keys.

App name: **Geda Clipboard** · Bundle ID: `com.geda.clipboard` · Team: `88BTYX26S4`

---

## 1. App information

| Field | Value |
| --- | --- |
| Name (≤30) | `Geda Clipboard` |
| Subtitle (≤30) | `Menu bar clipboard history` |
| Bundle ID | `com.geda.clipboard` |
| SKU | `geda-clipboard-mac` |
| Primary category | Productivity |
| Secondary category | Utilities |
| Primary language | English (U.S.) |
| Content rights | Does not contain, show, or access third-party content |
| Age rating | 4+ (every question in the questionnaire answered *None*) |

`LSApplicationCategoryType` in the bundle is `public.app-category.productivity`
and has to agree with the primary category above. A test in the root package
fails if the key goes missing.

## 2. Copyright

```
2026 Ân Vũ
```

No `©` symbol and no company suffix: App Store Connect adds the symbol itself,
and the field is rejected if the name does not match the legal entity on the
developer account.

## 3. URLs

| Field | Value |
| --- | --- |
| Privacy Policy URL (required) | `https://thienanblog.github.io/geda-clipboard/privacy.html` |
| Support URL (required) | `https://thienanblog.github.io/geda-clipboard/support.html` |
| Marketing URL (optional) | `https://thienanblog.github.io/geda-clipboard/` |
| EULA | Leave blank to use Apple's standard EULA. `terms.html` mirrors it plus the MIT licence for the GitHub build. |

All three are served by GitHub Pages from `docs/` on `main`, the same place the
Sparkle appcast is served from. **Confirm each one loads before submitting** —
a privacy policy URL that 404s is an automatic rejection.

## 4. Promotional text (≤170)

Editable without a new build, so this is the field to use for anything
time-sensitive.

```
Everything you copy, one shortcut away. Search it, preview it, and put it back on your clipboard in the app you were working in. Nothing ever leaves your Mac.
```

## 5. Description (≤4000)

```
Geda Clipboard keeps a searchable history of everything you copy, and puts it one keystroke away.

Press ⇧⌘V and the popup opens right where your pointer is, on whichever display it happens to be on. Type to narrow the list and press Return: the entry is on your clipboard and you are back in the app you were working in, one ⌘V from having it there.

It also tells you what it caught. Every copy, and every entry you pick out of the history, posts a notification showing the app it came from and a preview of the content, so you always know what is on your clipboard.

FEATURES

• Searchable history of text and images, with three thumbnail sizes
• The popup opens at the pointer, or under the menu bar icon, whichever you prefer
• Notifications on copy and on reuse, each one switchable on its own
• Choosing an entry returns you to the app you came from, so pasting is one ⌘V
• Copying the same thing again bumps the entry you already have and raises its counter, instead of filling the list with duplicates
• Local day, week, month and year statistics for text, images and repeated copies, stored as bounded counters rather than per-copy events
• Every entry records the app it came from, with its icon, and when you first and last copied it
• Point at any row for a card with the full text, its length, and its history
• Pin the entries you use every day, arrange their priority, and keep them safe when clearing routine history
• Keyboard throughout: ⌘1 to ⌘9 to pick an entry, ⌥P to pin, ⌥⌫ to delete, Esc to dismiss
• A global shortcut you can rebind to whatever is free on your Mac
• Launch at login, and no Dock icon unless you ask for one

PRIVATE BY DESIGN

Nothing Geda records ever leaves your Mac. There is no account, no sync, no telemetry, no third-party analytics, and no network connection of any kind. History and bounded usage counters live in the app's own sandboxed folder, and removing the app removes them.

Password managers are skipped automatically. They mark what they put on the clipboard as confidential, and Geda honours that convention. You can also name any application whose copies should never be recorded, cap how much history is kept, and clear the lot whenever you like.

PERMISSIONS

One, and it is optional: Notifications, so the copy and reuse alerts can reach you. Decline it and everything else still works; the alerts are simply not shown. Geda asks for nothing else. It does not use Accessibility, it reads no other application, and the keyboard shortcut that opens the popup needs no permission at all, so it works from first launch.

REQUIREMENTS

macOS 13 Ventura or later, on Apple silicon or Intel.

Geda Clipboard is free, with no in-app purchases and no advertising. It is open source, and the code behind every claim above can be read at github.com/thienanblog/geda-clipboard.
```

## 6. Keywords (≤100 characters including commas)

Commas with no spaces after them: the spaces count against the limit. Do not
repeat the app name or the category name; Apple already indexes both.

```
clipboard,copy,paste,history,pasteboard,snippet,menubar,shortcut,manager,clip,image,text
```

## 7. What's New in This Version (≤4000)

For the first submission this field is not shown. From the second version on,
paste that release's section from `CHANGELOG.md`, rewritten for users rather
than for the repository.

For version 0.10.0:

```
Pinned clipboard entries can now be arranged in a priority order from the new
Pinned preferences tab. New Compact, Comfortable and Large image preview sizes
let you choose how much detail the popup shows. Clearing history keeps pinned
entries by default, with an optional setting to include them.
```

## 8. App Privacy

Answer **"No, we do not collect data from this app."** That is the whole
questionnaire. It is accurate: the App Store build compiles without the
`sparkle` tag, links no framework that opens a socket, and contains no
telemetry, analytics or advertising SDK.

The build does declare `com.apple.security.network.client`, and it sends
nothing. WKWebView will not start under the sandbox without it, so the
entitlement buys the user interface rather than a connection; the frontend is
embedded in the binary and served over a custom scheme that never leaves the
process. Nothing here contradicts the answer above.

| Section | Answer |
| --- | --- |
| Data collection | None |
| Tracking | No |
| Third-party SDKs | None |

The nutrition label will read **Data Not Collected**.

## 9. Accessibility Nutrition Label

Claim only what the app actually does. Apple does not reject an app for
supporting few of these; it does reject one for claiming support it lacks, and
a reviewer with VoiceOver on will find out in under a minute.

| Feature | Declare | Why |
| --- | --- | --- |
| Reduced Motion | **Yes** | `prefers-reduced-motion: reduce` collapses every transition in the app, declared globally in `style.css` so it reaches the scoped component styles too. Deterministic from the stylesheet; no runtime testing needed to be sure of it. |
| Dark Interface | **Yes** | The interface is drawn dark and legible against the system dark appearance. |
| VoiceOver | **Test first** | The semantics are now correct — see below — but nobody has run VoiceOver against the app. Do not tick this box until somebody has. |
| Voice Control | **Test first** | Rides on the same labels VoiceOver uses, so the same test settles it. |
| Larger Text | **No** | Type sizes are fixed; the UI does not follow the system text size. |
| Sufficient Contrast | **No** | Not measured. Do not claim it until every foreground/background pair in `style.css` has been checked against 4.5:1. |
| Differentiate Without Colour | **No** | The selected row is still distinguished visually by fill colour alone. Its current-state semantics help assistive technology, not a sighted user who cannot separate the two colours. |
| Captions / Audio Descriptions | N/A | The app plays no media. |

**Current implementation.** The search field has an accessible name and
publishes the highlighted primary row action through `aria-activedescendant`,
so selection can be spoken while the caret stays in search. History is an
explicit list whose rows describe image dimensions and source applications.
Each row exposes paste/copy and pin/unpin as separate labelled buttons, avoiding
nested interactive controls.

**The two-minute test before ticking VoiceOver or Voice Control.** Press
`⌘F5`, open the popup, and check that: the search field announces itself; the
arrow keys move the selection and each row is read out, including the image
row; `⌘1` activates the right entry; the footer buttons and the whole of
Preferences are reachable and announced. If all of that holds, the claim is
honest. If any of it does not, fix it or leave the box unticked — Apple does
not reject an app for supporting few of these, and does reject one for claiming
support it lacks.

## 10. Pricing and Availability

| Field | Value |
| --- | --- |
| Price | Free (Price Tier 0) |
| Availability | All territories |
| Pre-orders | No |
| Distribution | Public on the Mac App Store |
| Educational discount | Not applicable to free apps |

## 11. Screenshots

macOS accepts 1280×800, 1440×900, 2560×1600 or 2880×1800, between one and ten
of them. Six are supplied at **2880×1800**, generated by
`scripts/screenshot-fixture` plus the composition step described in
`scripts/README.md`:

| File | Shows |
| --- | --- |
| `01-history.png` | The popup with a full history, pinned entries at the top |
| `02-preview.png` | The detail card floating beside the list |
| `03-search.png` | Searching by source application |
| `04-preferences.png` | General settings, including notifications and the shortcut |
| `05-privacy.png` | Confidential-copy skips, the ignore list and local-data controls |
| `06-statistics.png` | Synthetic local copy totals and the Statistics line chart |

Every entry in them comes from a synthetic fixture. No real clipboard content
appears in any screenshot, which is not a nicety: a clipboard manager's
screenshots are exactly where a real secret would end up on a public store page.

The app icon is taken from the bundle's `iconfile.icns`, built from
`build/appicon.png` at 1024×1024. macOS icons may carry alpha, unlike iOS.

## 12. App Review Information

| Field | Value |
| --- | --- |
| Sign-in required | No |
| Demo account | Not needed |
| Contact | The developer account's own name, email and phone |

Notes for the reviewer:

```
Geda Clipboard is a menu bar utility. It has no window of its own and no Dock icon, so nothing appears when it is launched: its icon is in the menu bar at the top right of the screen. Click that icon, or press Shift-Command-V, to open the popup.

Copy some text in any app first so the history has something in it.

The app asks for one optional permission: Notifications, for the alert shown when something is copied or reused. It can be declined and the app remains fully functional apart from that alert.

This build requests no Accessibility permission and contains no Accessibility code. Submission 989d447d (version 0.7.0, build 5) was rejected under guideline 2.4.5 because the app used Accessibility to send a Command-V keystroke. That feature has been removed from this build entirely, along with the preferences that offered it: the keystroke path is compiled out, and the binary references no Accessibility API. Choosing an entry now copies it to the clipboard and returns the user to the application they were working in, and they press Command-V themselves.

The app makes no network connections. It has no account, no telemetry, no third-party analytics and no third-party SDKs, and this build contains no auto-update mechanism. The Statistics tab aggregates copy counts entirely on the device. It stores no clipboard content, hashes, source-application names or per-copy event log, automatically discards totals older than 370 days, and can be reset independently under Preferences. This on-device processing is not transmitted to us or any third party.
```

## 13. Export compliance

`ITSAppUsesNonExemptEncryption` is already `false` in the bundle's `Info.plist`,
so App Store Connect stops asking on every upload. The app ships no
cryptography of its own and opens no sockets.

## 14. Version and build rules

The rule that costs the most to learn the hard way: **a marketing version that
has already been released can never be submitted again.** Raising only the
build number does not reopen it. `scripts/package-appstore.sh` runs
`scripts/asc` before it builds, which asks App Store Connect and refuses early.

- `CFBundleShortVersionString` comes from `wails.json` and must match `appVersion`.
- `CFBundleVersion` comes from `BUILD_NUMBER` and must be strictly higher than
  every build already uploaded for this macOS app, including builds under older
  marketing versions. It does not reset for a new version.

```bash
AC_API_KEY_P8=~/path/AuthKey_XXXXXXXXXX.p8 \
AC_API_KEY_ID=XXXXXXXXXX \
AC_API_ISSUER_ID=<issuer uuid from Users and Access, Integrations> \
BUILD_NUMBER=NEXT_BUILD_NUMBER \
APP_IDENTITY="3rd Party Mac Developer Application: An Vu (88BTYX26S4)" \
INSTALLER_IDENTITY="3rd Party Mac Developer Installer: An Vu (88BTYX26S4)" \
PROFILE=~/path/Geda_Clipboard_MAS.provisionprofile \
  ./scripts/package-appstore.sh
```

Upload the resulting `.pkg` with Transporter.
