#ifndef GEDA_WINDOW_DARWIN_H
#define GEDA_WINDOW_DARWIN_H

// Places the application's main window at a global, top-left-origin position,
// leaving its size alone. Returns 1 when the window was found and moved.
int gedaMoveWindow(int x, int y);

// Shrinks the frosted window material to the panel: everything left of the
// inset is left transparent, and the material is rounded to match the panel's
// corners.
void gedaSetPanelInset(int left, int radius);

#endif
