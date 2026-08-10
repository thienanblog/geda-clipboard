#ifndef GEDA_CLIPBOARD_DARWIN_H
#define GEDA_CLIPBOARD_DARWIN_H

// All returned char*/void* buffers are malloc'd and owned by the caller.

long long gedaChangeCount(void);

// Reads the pasteboard. kind: 0 none, 1 text, 2 image (PNG in img/imgLen).
void gedaRead(int *kind, char **text, void **img, int *imgLen,
              int *concealed, int *transient, int *remote);

long long gedaWriteText(const char *text);
long long gedaWriteImage(const void *bytes, int len);

void gedaFrontmost(char **name, char **bundleID);
void *gedaAppIconPNG(const char *bundleID, int px, int *outLen);

void gedaRememberFrontmost(void);
int gedaPaste(void);
int gedaHasAccessibility(int prompt);

#endif
