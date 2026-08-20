#ifndef GEDA_CLIPBOARD_PASTE_DARWIN_H
#define GEDA_CLIPBOARD_PASTE_DARWIN_H

// The synthetic-keystroke paste path. Everything here needs Accessibility
// permission, which is why the whole file is compiled only into builds made
// with the axpaste tag. The Mac App Store build is made without it: guideline
// 2.4.5 forbids using Accessibility to automate other applications, and Apple
// inspects the shipped binary rather than the source.

// Sends the paste keystroke to the remembered application. Returns 0 on
// success and -1 when Accessibility permission is missing.
int gedaPaste(void);

// Reports whether this process is trusted for Accessibility. When prompt is
// non-zero macOS may show its one-time grant dialog.
int gedaHasAccessibility(int prompt);

// Reveals the Accessibility list in System Settings. Returns 1 when the pane
// was opened.
int gedaOpenAccessibilitySettings(void);

#endif
