export type SettingsTab = 'general' | 'clipboard' | 'pinned' | 'privacy' | 'statistics' | 'about'

export const settingsTabs: Array<{ value: SettingsTab; label: string }> = [
  { value: 'general', label: 'General' },
  { value: 'clipboard', label: 'Clipboard' },
  { value: 'pinned', label: 'Pinned' },
  { value: 'privacy', label: 'Privacy' },
  { value: 'statistics', label: 'Statistics' },
  { value: 'about', label: 'About' },
]

export interface SettingLink {
  id: string
  tab: SettingsTab
  label: string
  description: string
  keywords: string
  requires?: 'mac' | 'paste' | 'updates' | 'pastePermission'
}

// Search describes controls, never clipboard contents or the ignored-app list.
export const settingLinks: SettingLink[] = [
  { id: 'notify-copy', tab: 'general', label: 'Notify when something is copied', description: 'Turn copy notifications on or off.', keywords: 'notifications alerts thong bao sao chep' },
  { id: 'notify-paste', tab: 'general', label: 'Notify when an entry is reused', description: 'Turn notifications for chosen entries on or off.', keywords: 'notifications alerts paste pasted dan thong bao' },
  { id: 'notification-preview', tab: 'general', label: 'Show content in notifications', description: 'Hide text, file names and image details while keeping alerts and app names.', keywords: 'privacy notifications preview sensitive hidden noi dung thong bao rieng tu' },
  { id: 'notification-test', tab: 'general', label: 'Notification permission and test', description: 'Check permissions or send a test notification.', keywords: 'notifications blocked allow permission thong bao quyen' },
  { id: 'hotkey', tab: 'general', label: 'Global shortcut', description: 'Change the key combination that opens the popup.', keywords: 'shortcut keyboard hotkey toggle phim tat' },
  { id: 'launch', tab: 'general', label: 'Launch at login', description: 'Start Geda automatically when you sign in.', keywords: 'application startup boot khoi dong' },
  { id: 'dock', tab: 'general', label: 'Show icon in the Dock', description: 'Keep Geda visible in the macOS Dock.', keywords: 'application icon bieu tuong', requires: 'mac' },
  { id: 'hover', tab: 'general', label: 'Show details on hover', description: 'Show a detail card when pointing at an entry.', keywords: 'window appearance preview mouse chuot xem truoc' },
  { id: 'placement', tab: 'general', label: 'Popup position', description: 'Open at the pointer or the menu bar / tray icon.', keywords: 'window appearance location placement vi tri cua so' },
  { id: 'size', tab: 'general', label: 'Popup size', description: 'Adjust the width and height of the history window.', keywords: 'window appearance dimensions pixels kich thuoc rong cao' },
  { id: 'image-size', tab: 'general', label: 'Image preview size', description: 'Choose Compact, Comfortable or Large thumbnails.', keywords: 'window appearance images thumbnail anh kich thuoc' },
  { id: 'reset', tab: 'general', label: 'Reset all preferences', description: 'Restore default settings without deleting history.', keywords: 'defaults restore reset khoi phuc mac dinh' },
  { id: 'paste', tab: 'clipboard', label: 'Paste immediately when an entry is chosen', description: 'Choose between automatic paste and copying for a manual paste.', keywords: 'behaviour select reuse dan tu dong', requires: 'paste' },
  { id: 'capture-images', tab: 'clipboard', label: 'Record images', description: 'Include new image copies in history.', keywords: 'behaviour capture images screenshot anh chup' },
  { id: 'capture-files', tab: 'clipboard', label: 'Record files and folders', description: 'Save references to copied files. Existing history is kept.', keywords: 'behaviour capture files folders finder explorer tep thu muc' },
  { id: 'move-top', tab: 'clipboard', label: 'Move chosen entries to the top', description: 'Control history order after copying or pasting a saved entry.', keywords: 'history order sort recent reuse lich su sap xep' },
  { id: 'history-limit', tab: 'clipboard', label: 'History limit', description: 'Keep 10–2,000 unpinned entries. Pins are always kept.', keywords: 'history maximum storage capacity keep entries gioi han lich su' },
  { id: 'paste-permission', tab: 'clipboard', label: 'Accessibility permission', description: 'Allow Geda to paste into the previous application.', keywords: 'paste permission grant access quyen dan', requires: 'pastePermission' },
  { id: 'pin-priority', tab: 'pinned', label: 'Pinned entry priority', description: 'Reorder pins or return them to automatic order.', keywords: 'priority favourites favorites order ghim sap xep' },
  { id: 'clear-pins', tab: 'pinned', label: 'Clear pinned entries when clearing history', description: 'Choose whether clearing history also removes pins.', keywords: 'history delete clear pins protect xoa ghim' },
  { id: 'confidential', tab: 'privacy', label: 'Skip entries marked confidential', description: 'Respect the exclusion requested by password managers.', keywords: 'password secret sensitive concealed bao mat mat khau' },
  { id: 'transient', tab: 'privacy', label: 'Skip entries marked temporary', description: 'Ignore transient clipboard data from source apps.', keywords: 'transient temporary tam thoi' },
  { id: 'ignored-apps', tab: 'privacy', label: 'Never record copies from these apps', description: 'Exclude applications by name, one per line.', keywords: 'ignored apps exclude block blacklist ung dung bo qua' },
  { id: 'clear-data', tab: 'privacy', label: 'Clear history and statistics', description: 'Delete local history and usage counters after confirmation.', keywords: 'local data delete storage erase reset xoa du lieu lich su' },
  { id: 'statistics', tab: 'statistics', label: 'Copy statistics', description: 'View day, week, month and year charts or reset local counters.', keywords: 'statistics reset clear charts counts usage thong ke bieu do' },
  { id: 'about', tab: 'about', label: 'About, support and privacy policy', description: 'Find the app version, platform, help and legal links.', keywords: 'about version support terms privacy help phien ban ho tro' },
  { id: 'updates', tab: 'about', label: 'Check for updates', description: 'Look for a newer version of Geda Clipboard.', keywords: 'update upgrade version cap nhat', requires: 'updates' },
]

export interface SearchCapabilities {
  isMac: boolean
  pasteSupported: boolean
  canPaste: boolean
  canUpdate: boolean
}

function normalise(value: string): string {
  return value.normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/đ/g, 'd').replace(/Đ/g, 'D').toLowerCase()
}

export function searchSettings(query: string, capabilities: SearchCapabilities): SettingLink[] {
  const words = normalise(query).trim().split(/\s+/).filter(Boolean)
  if (!words.length) return []
  return settingLinks.filter((item) => {
    if (item.requires === 'mac' && !capabilities.isMac) return false
    if (item.requires === 'paste' && !capabilities.pasteSupported) return false
    if (item.requires === 'pastePermission' && (!capabilities.pasteSupported || capabilities.canPaste)) return false
    if (item.requires === 'updates' && !capabilities.canUpdate) return false
    const text = normalise(`${item.tab} ${item.label} ${item.description} ${item.keywords}`)
    return words.every((word) => text.includes(word))
  })
}
