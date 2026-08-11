#ifndef GEDA_AUTOSTART_DARWIN_H
#define GEDA_AUTOSTART_DARWIN_H

// Registers or unregisters the running bundle as a login item. Returns 0 on
// success; on failure returns -1 and, when errOut is non-NULL, stores a
// malloc'd message the caller owns.
int gedaLoginItemSet(int enabled, char **errOut);

// Reports whether the bundle is currently registered to launch at login.
int gedaLoginItemEnabled(void);

#endif
