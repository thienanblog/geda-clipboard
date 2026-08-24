#ifndef GEDA_CLIPBOARD_DARWIN_H
#define GEDA_CLIPBOARD_DARWIN_H

// All returned char*/void* buffers are malloc'd and owned by the caller.
//
// Nothing declared here touches Accessibility. The keystroke path lives in
// clipboard_paste_darwin.h, behind the axpaste build tag, so a build made
// without that tag carries no Accessibility symbol at all.

long long gedaChangeCount(void);

// Reads the pasteboard. kind: 0 none, 1 text, 2 image (PNG in img/imgLen).
// pending is set when a supported type is declared but its data is not ready.
void gedaRead(int *kind, char **text, void **img, int *imgLen,
              int *concealed, int *transient, int *remote, int *pending);

long long gedaWriteText(const char *text);
long long gedaWriteImage(const void *bytes, int len);

void gedaFrontmost(char **name, char **bundleID);
void *gedaAppIconPNG(const char *bundleID, int px, int *outLen);

void gedaRememberFrontmost(void);

// Refocuses the remembered application. Returns 1 when it holds focus.
int gedaActivateRemembered(void);

#endif
