#ifndef GEDA_CLIPBOARD_DARWIN_H
#define GEDA_CLIPBOARD_DARWIN_H

#include <stdint.h>

// All returned char*/void* buffers are malloc'd and owned by the caller.
//
// Nothing declared here touches Accessibility. The keystroke path lives in
// clipboard_paste_darwin.h, behind the axpaste build tag, so a build made
// without that tag carries no Accessibility symbol at all.

long long gedaChangeCount(void);

// Reads the pasteboard. kind: 0 none, 1 text, 2 image (PNG in img/imgLen),
// 3 file group (JSON array of path/bookmark objects in filesJSON).
// pending is set when a supported type is declared but its data is not ready.
void gedaRead(int *kind, char **text, void **img, int *imgLen, char **filesJSON,
              int *concealed, int *transient, int *remote, int *pending);

long long gedaWriteText(const char *text);
long long gedaWriteImage(const void *bytes, int len);
long long gedaWriteFiles(const char *filesJSON);

// Resolves and starts access to a security-scoped bookmark. The returned token
// is owned by the caller and must be passed to gedaStopFileAccess. resolvedPath
// and errorMessage are malloc'd strings owned by the caller.
void *gedaStartFileAccess(const char *bookmark, char **resolvedPath, char **errorMessage);
void gedaStopFileAccess(void *token);

void gedaFrontmost(char **name, char **bundleID);
void *gedaAppIconPNG(const char *bundleID, int px, int *outLen);

void gedaRememberFrontmost(void);

// Read before hiding the popup. A previous target must exist and Geda must
// still be frontmost; a click elsewhere must not start a new focus return.
int gedaCanBeginFocusReturn(void);

// Window Server mouse-down count, used to cancel a delayed action after a
// click even when the destination shares the remembered application's PID.
uint64_t gedaMouseDownCount(void);

// True only while our app or the remembered app is frontmost.
int gedaCanRestoreFocus(void);

// True only while the remembered app can receive a paste keystroke.
int gedaRememberedIsFrontmost(void);

// Refocuses the remembered application. Returns 1 when it holds focus.
int gedaActivateRemembered(void);

#endif
