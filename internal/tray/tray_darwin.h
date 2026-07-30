#ifndef GEDA_TRAY_DARWIN_H
#define GEDA_TRAY_DARWIN_H

void gedaTrayCreate(const void *iconBytes, int iconLen, const char *tooltip);
void gedaTrayDestroy(void);

// Reports the state of the status item: 1 when it exists, 0 otherwise.
int gedaTrayExists(void);

#endif
