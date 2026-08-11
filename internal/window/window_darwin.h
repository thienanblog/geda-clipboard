#ifndef GEDA_WINDOW_DARWIN_H
#define GEDA_WINDOW_DARWIN_H

// Places the application's main window at a global, top-left-origin position,
// leaving its size alone. Returns 1 when the window was found and moved.
int gedaMoveWindow(int x, int y);

// Shrinks the frosted window material to the panel: everything outside the
// insets is left transparent, and the material is rounded to match the panel's
// corners. Also suppresses the native window shadow, which cannot be drawn
// correctly for a translucent WebView window.
void gedaSetPanelInset(int left, int top, int right, int bottom, int radius);

// Shows or hides the Dock tile by switching the process between the regular
// and accessory activation policies.
void gedaSetDockIconVisible(int visible);

#endif
