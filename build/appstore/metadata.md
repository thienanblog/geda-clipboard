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
Everything you copy, one shortcut away. Search it, preview it, and paste it straight back into the app you were working in. Nothing ever leaves your Mac.
```

## 5. Description (≤4000)

```
Geda Clipboard keeps a searchable history of everything you copy, and puts it one keystroke away.

Press ⇧⌘V and the popup opens right where your pointer is, on whichever display it happens to be on. Type to narrow the list, press Return, and the entry goes straight back into the app you were working in.

It also tells you what it caught. Every copy and every paste posts a notification showing the app it came from and a preview of the content, so you always know what is on your clipboard.

FEATURES

• Searchable history of text and images, with thumbnails
• The popup opens at the pointer, or under the menu bar icon, whichever you prefer
• Notifications on copy and on paste, each one switchable on its own
• Pastes back into the app you came from, without you reaching for ⌘V
• Copying the same thing again bumps the entry you already have and raises its counter, instead of filling the list with duplicates
• Every entry records the app it came from, with its icon, and when you first and last copied it
• Point at any row for a card with the full text, its length, and its history
• Pin the entries you use every day so they outlive the history limit
• Keyboard throughout: ⌘1 to ⌘9 to pick an entry, ⌥P to pin, ⌥⌫ to delete, Esc to dismiss
• A global shortcut you can rebind to whatever is free on your Mac
• Launch at login, and no Dock icon unless you ask for one

PRIVATE BY DESIGN

Nothing Geda records ever leaves your Mac. There is no account, no sync, no analytics, and no network connection of any kind. The history lives in the app's own sandboxed folder, and removing the app removes it.

Password managers are skipped automatically. They mark what they put on the clipboard as confidential, and Geda honours that convention. You can also name any application whose copies should never be recorded, cap how much history is kept, and clear the lot whenever you like.

PERMISSIONS

Notifications, so the copy and paste alerts can reach you. Accessibility, so Geda can press ⌘V on your behalf and land the entry back where you were typing. Both are optional: without them the app still keeps your history and still puts entries on the clipboard for you to paste yourself.

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

## 8. App Privacy

Answer **"No, we do not collect data from this app."** That is the whole
questionnaire. It is accurate: the App Store build compiles without the
`sparkle` tag, links no framework that opens a socket, declares no network
entitlement, and contains no analytics or advertising SDK.

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
| Dark Interface | **Yes** | The interface is drawn dark and legible against the system dark appearance. |
| VoiceOver | **No** | Rows and footer actions are buttons and do announce, but the search field has no label, the list carries no listbox semantics, and the selected row is not announced as selected. Partial support is not support. |
| Voice Control | **No** | Depends on the same missing labels. |
| Larger Text | **No** | Type sizes are fixed; the UI does not follow the system text size. |
| Sufficient Contrast | **No** | Not measured. Do not claim it until the palette has been checked against 4.5:1. |
| Reduced Motion | **No** | The popup and the detail card fade in without consulting `prefers-reduced-motion`. |
| Differentiate Without Colour | **No** | Selection is shown by fill colour alone. |
| Captions / Audio Descriptions | N/A | The app plays no media. |

These are worth fixing, and most are small: labelling the search field, giving
the list `role="listbox"` with `aria-selected` on rows, and honouring
`prefers-reduced-motion` would between them make VoiceOver and Reduced Motion
honest claims.

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
of them. Five are supplied at **2880×1800**, generated by
`scripts/screenshot-fixture` plus the composition step described in
`scripts/README.md`:

| File | Shows |
| --- | --- |
| `01-history.png` | The popup with a full history, pinned entries at the top |
| `02-preview.png` | The detail card floating beside the list |
| `03-search.png` | Searching by source application |
| `04-preferences.png` | Notifications, behaviour and shortcut settings |
| `05-privacy.png` | History limit, the confidential-copy skip, and the ignore list |

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

The app asks for two optional permissions. Notifications are used for the alert shown on each copy and paste. Accessibility is used only to send a Command-V keystroke so a chosen entry lands back in the app the user was working in; it is not used to read other applications. Both can be declined and the app remains fully functional apart from those two conveniences.

The app makes no network connections. It has no account, no analytics and no third-party SDKs, and this build contains no auto-update mechanism.
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
  any build already uploaded for that marketing version.
- For a new marketing version, `BUILD_NUMBER` may start again at 1.

```bash
AC_API_KEY_P8=/Users/anvu/simple-projects/geda-secrets/AuthKey_X8W6RC9J9P.p8 \
AC_API_KEY_ID=X8W6RC9J9P \
AC_API_ISSUER_ID=<issuer uuid from Users and Access, Integrations> \
BUILD_NUMBER=1 \
APP_IDENTITY="3rd Party Mac Developer Application: An Vu (88BTYX26S4)" \
INSTALLER_IDENTITY="3rd Party Mac Developer Installer: An Vu (88BTYX26S4)" \
PROFILE=/Users/anvu/simple-projects/geda-secrets/Geda_Clipboard.provisionprofile \
  ./scripts/package-appstore.sh
```

Upload the resulting `.pkg` with Transporter.
