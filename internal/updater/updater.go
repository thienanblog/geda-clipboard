// Package updater exposes the in-app update check.
//
// The implementation behind it is chosen at build time. Building with
// -tags sparkle links Sparkle and gives the app a real updater; building
// without the tag, which is what the App Store package does, compiles to a
// stub that reports the feature as unavailable and pulls in no framework.
//
// The tag adds Sparkle rather than removing it, and that direction is
// deliberate. A subtractive tag fails towards shipping an auto-updater to the
// App Store, which is a rejection under guideline 2.4.5 and is invisible in
// the source. An additive tag fails towards a build with no updater, which is
// merely a missing menu item. When a flag is going to be forgotten sooner or
// later, it should be forgotten in the harmless direction.
package updater

// Available reports whether this build has an updater compiled into it.
func Available() bool { return available() }

// Start brings the updater up and schedules its background checks. It is a
// no-op in builds without one.
func Start() { start() }

// CheckNow runs a user-initiated check, showing the updater's own UI. It is a
// no-op in builds without an updater; callers should hide the menu item when
// Available reports false rather than relying on that.
func CheckNow() { checkNow() }
