#ifndef GEDA_TRAY_DARWIN_H
#define GEDA_TRAY_DARWIN_H

void gedaTrayCreate(const void *iconBytes, int iconLen, const char *tooltip);
void gedaTrayDestroy(void);

// Reports the state of the status item: 1 when it exists, 0 otherwise.
int gedaTrayExists(void);

// Reports the mouse pointer relative to the work area of the screen holding
// it, plus that work area in global coordinates. Returns 1 on success.
int gedaCursorAnchor(int *x, int *y, int *workX, int *workY, int *workW, int *workH);

#endif
