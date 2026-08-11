#ifndef GEDA_UPDATER_SPARKLE_DARWIN_H
#define GEDA_UPDATER_SPARKLE_DARWIN_H

// Creates the updater and starts its scheduled background checks. Safe to
// call more than once; only the first call does anything.
void gedaUpdaterStart(void);

// Runs a user-initiated check with Sparkle's own progress and release-notes
// UI. Starts the updater first if it is not running yet.
void gedaUpdaterCheckNow(void);

#endif
