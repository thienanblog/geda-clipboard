#ifndef GEDA_WINDOW_DARWIN_H
#define GEDA_WINDOW_DARWIN_H

// Places the application's main window at a global, top-left-origin position
// and sizes it. Returns 1 when the window was found and moved, 0 otherwise.
int gedaMoveWindow(int x, int y, int w, int h);

#endif
