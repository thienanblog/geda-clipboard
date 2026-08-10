#ifndef GEDA_WINDOW_DARWIN_H
#define GEDA_WINDOW_DARWIN_H

// Places the application's main window at a global, top-left-origin position,
// leaving its size alone. Returns 1 when the window was found and moved.
int gedaMoveWindow(int x, int y);

#endif
