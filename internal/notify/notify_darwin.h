#ifndef GEDA_NOTIFY_DARWIN_H
#define GEDA_NOTIFY_DARWIN_H

// Returns 1 when the process is a bundled app and can use the modern
// UserNotifications framework.
int gedaNotifyBundled(void);

void gedaNotifyRequestPermission(void);
int gedaNotifySend(const char *title, const char *subtitle, const char *body);

// Authorization status, matching UNAuthorizationStatus:
// 0 not determined, 1 denied, 2 authorized, 3 provisional, 4 ephemeral.
// Returns -1 when the framework is unavailable (unbundled or pre-10.14).
int gedaNotifyAuthStatus(void);

#endif
