<script lang="ts" setup>
/** Tabbed preferences and About. Changes are saved as soon as they are made,
 *  which is what a menu bar utility should do -- there is no OK/Cancel. */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { main, settings } from '../../wailsjs/go/models'
import * as App from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { eventToSpec, formatHotkey } from '../lib/keys'
import StatisticsView from './StatisticsView.vue'

type SettingsTab = 'general' | 'clipboard' | 'privacy' | 'statistics' | 'about'

const settingsTabs: Array<{ value: SettingsTab; label: string }> = [
  { value: 'general', label: 'General' },
  { value: 'clipboard', label: 'Clipboard' },
  { value: 'privacy', label: 'Privacy' },
  { value: 'statistics', label: 'Statistics' },
  { value: 'about', label: 'About' },
]

const privacyURL = 'https://thienanblog.github.io/geda-clipboard/privacy.html'
const supportURL = 'https://thienanblog.github.io/geda-clipboard/support.html'
const termsURL = 'https://thienanblog.github.io/geda-clipboard/terms.html'

const props = defineProps<{
  tab: SettingsTab
  env: main.Environment | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'tab', value: SettingsTab): void
}>()

const cfg = ref<settings.Settings | null>(null)
const status = ref('')
const capturingHotkey = ref(false)
const canPaste = ref(true)
/** False in the Mac App Store build, which has no paste-back path: the
 *  Accessibility permission it would need may not be used to automate other
 *  applications. Every control that talks about that permission is hidden
 *  rather than shown as unavailable, because there is nothing to grant. */
const pasteSupported = ref(true)
const hotkeyError = ref('')
const notifyStatus = ref('unknown')
let statusTimer: number | undefined

/** Notifications are the point of the app, so a denied permission is worth
 *  calling out rather than letting alerts vanish silently. */
const notifyBlocked = computed(
  () => notifyStatus.value === 'denied' || notifyStatus.value === 'notDetermined',
)

const notifyMessage = computed(() =>
  notifyStatus.value === 'denied'
    ? 'Notifications are turned off for Geda Clipboard, so copy and paste alerts will not appear.'
    : 'Notification permission has not been granted yet, so copy and paste alerts may not appear.',
)

/** Newline-separated editing buffer for the ignored-apps list. */
const ignoredText = ref('')

const hotkeyLabel = computed(() =>
  capturingHotkey.value ? 'Press keys…' : formatHotkey(cfg.value?.hotkey ?? ''),
)

/** The icon lives in the menu bar on macOS and the notification area elsewhere,
 *  and the placement option has to name the one the user can actually see. */
const iconPlacementLabel = computed(() =>
  props.env?.platform === 'darwin' ? 'The menu bar icon' : 'The tray icon',
)

/** Only macOS has a Dock, so the option to leave it alone is hidden elsewhere. */
const isMac = computed(() => props.env?.platform === 'darwin')

/** False in the App Store build, which has no updater compiled in: updates
 *  arrive through the App Store instead. Hiding the control is the honest
 *  thing -- a button that cannot do anything is worse than no button. */
const canUpdate = ref(false)

async function load(): Promise<void> {
  cfg.value = await App.GetSettings()
  ignoredText.value = (cfg.value.ignoredApps ?? []).join('\n')
  canPaste.value = props.env?.canPaste ?? true
  pasteSupported.value = props.env?.pasteSupported ?? true
  hotkeyError.value = props.env?.hotkeyError ?? ''
  try {
    notifyStatus.value = await App.NotificationStatus()
  } catch {
    notifyStatus.value = 'unknown'
  }
  canUpdate.value = await App.UpdatesSupported()
}

/** macOS never tells a running application that a permission was granted, so
 *  both warnings would otherwise stay up until the next launch. Preferences are
 *  where the user goes to fix this and where they come back to, so re-reading
 *  the state on focus is enough to clear them at the moment they look. */
async function refreshPermissions(): Promise<void> {
  if (pasteSupported.value) {
    try {
      canPaste.value = await App.PastePermission()
    } catch {
      // Leave the last known answer in place; a failed check is not a denial.
    }
  }
  try {
    notifyStatus.value = await App.NotificationStatus()
  } catch {
    // As above.
  }
}

async function openPasteSettings(): Promise<void> {
  try {
    await App.OpenPastePermissionSettings()
    flash('Add Geda Clipboard to the Accessibility list, then return here')
  } catch (err) {
    flash(String(err))
  }
}

async function fixNotifications(): Promise<void> {
  notifyStatus.value = await App.RequestNotificationPermission()
  if (notifyStatus.value === 'denied') {
    flash('Enable Geda Clipboard in System Settings › Notifications')
  }
}

async function sendTest(): Promise<void> {
  try {
    await App.SendTestNotification()
    flash('Test notification sent')
  } catch (err) {
    flash(String(err))
  }
}

function flash(message: string): void {
  status.value = message
  window.clearTimeout(statusTimer)
  statusTimer = window.setTimeout(() => (status.value = ''), 2000)
}

async function save(): Promise<void> {
  if (!cfg.value) return
  try {
    // The Go side normalises and returns the stored values, so adopt its answer
    // rather than assuming ours was accepted verbatim.
    const stored = await App.SaveSettings(cfg.value)
    cfg.value = stored
    ignoredText.value = (stored.ignoredApps ?? []).join('\n')
    flash('Saved')
  } catch (err) {
    flash(String(err))
  }
}

/** Commits a numeric field. Vue's `.number` modifier leaves the raw string in
 *  place when it cannot be parsed, so an emptied field would send "" where Go
 *  expects an int -- which fails to unmarshal and then blocks every later save
 *  too, because the bad value stays in cfg. Fall back to the stored value and
 *  let Go clamp the rest. */
function onNumberChange(field: 'maxItems' | 'popupWidth' | 'popupHeight'): void {
  if (!cfg.value) return
  // The runtime type is whatever the modifier produced, not necessarily number.
  const raw = cfg.value[field] as unknown
  const value = typeof raw === 'number' ? raw : Number(String(raw).trim())
  if (!Number.isFinite(value) || String(raw).trim() === '') {
    void load() // discard the unusable input, restore what is on disk
    return
  }
  cfg.value[field] = Math.round(value)
  void save()
}

function onIgnoredInput(): void {
  if (!cfg.value) return
  cfg.value.ignoredApps = ignoredText.value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
  void save()
}

async function resetDefaults(): Promise<void> {
  cfg.value = await App.DefaultSettings()
  await save()
}

async function clearHistoryAndStatistics(): Promise<void> {
  if (!window.confirm('Clear all clipboard history and local statistics? This cannot be undone.')) return
  try {
    await App.ClearHistoryAndStatistics()
    flash('History and statistics cleared')
  } catch (err) {
    flash(String(err))
  }
}

function startHotkeyCapture(): void {
  capturingHotkey.value = true
}

function onHotkeyKeydown(event: KeyboardEvent): void {
  if (!capturingHotkey.value || !cfg.value) return
  event.preventDefault()
  event.stopPropagation()

  if (event.key === 'Escape') {
    capturingHotkey.value = false
    return
  }

  const spec = eventToSpec(event)
  if (!spec) return // modifier-only press: keep waiting for a real key

  cfg.value.hotkey = spec
  capturingHotkey.value = false
  void saveHotkey()
}

/** Saving a shortcut is the one setting that can be refused by something
 *  outside the app -- another application already holding the combination --
 *  so the stored value alone does not say whether it took effect. Ask what the
 *  app actually holds now and report that instead. */
async function saveHotkey(): Promise<void> {
  await save()
  try {
    hotkeyError.value = (await App.Env()).hotkeyError
  } catch {
    hotkeyError.value = ''
  }
}

/** Raises the system prompt, then falls back to the settings pane. macOS only
 *  shows that prompt once per application, so on every later attempt this call
 *  returns false without displaying anything -- which is why the pane is opened
 *  rather than leaving a button that appears to do nothing. */
async function requestPastePermission(): Promise<void> {
  canPaste.value = await App.RequestPastePermission()
  if (!canPaste.value) {
    await openPasteSettings()
  }
}

function onKeydown(event: KeyboardEvent): void {
  if (capturingHotkey.value) {
    onHotkeyKeydown(event)
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
  }
}

onMounted(() => {
  void load()
  window.addEventListener('keydown', onKeydown, true)
  window.addEventListener('focus', refreshPermissions)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown, true)
  window.removeEventListener('focus', refreshPermissions)
  window.clearTimeout(statusTimer)
})
</script>

<template>
  <div class="panel">
    <header class="header">
      <nav class="tabs">
        <button
          v-for="item in settingsTabs"
          :key="item.value"
          class="tab"
          :class="{ active: tab === item.value }"
          type="button"
          @click="emit('tab', item.value)"
        >
          {{ item.label }}
        </button>
      </nav>

      <div class="header-right">
        <span v-if="status" class="status">{{ status }}</span>
        <button class="close" type="button" title="Close" @click="emit('close')">Done</button>
      </div>
    </header>

    <div class="hairline" />

    <div class="body scroll">
      <template v-if="tab === 'general' && cfg">
        <section>
          <h2>Notifications</h2>
          <p v-if="notifyBlocked" class="hint warn">{{ notifyMessage }}</p>
          <div v-if="notifyBlocked" class="btn-row">
            <button class="btn" type="button" @click="fixNotifications">
              {{ notifyStatus === 'denied' ? 'Open notification settings…' : 'Allow notifications' }}
            </button>
          </div>
          <label class="check">
            <input v-model="cfg.notifyOnCopy" type="checkbox" @change="save" />
            <span>Notify when something is copied<em>Shows the source app and a preview.</em></span>
          </label>
          <label class="check">
            <input v-model="cfg.notifyOnPaste" type="checkbox" @change="save" />
            <span v-if="pasteSupported">Notify when an entry is pasted<em>Confirms which app received the paste.</em></span>
            <span v-else>Notify when an entry is reused<em>Confirms the entry is ready to paste.</em></span>
          </label>
          <div class="btn-row">
            <button class="btn" type="button" @click="sendTest">Send a test notification</button>
          </div>
        </section>

        <section>
          <h2>Shortcut</h2>
          <div class="field">
            <span class="field-label">Toggle the popup</span>
            <button class="hotkey" :class="{ capturing: capturingHotkey }" type="button" @click="startHotkeyCapture">
              {{ hotkeyLabel }}
            </button>
          </div>
          <p v-if="hotkeyError" class="hint warn">
            This shortcut is not active: {{ hotkeyError }}. Pick a different combination;
            {{ iconPlacementLabel.toLowerCase() }} opens the popup in the meantime.
          </p>
          <p class="hint">Click the shortcut, then press the key combination you want.</p>
        </section>

        <section>
          <h2>Application</h2>
          <label class="check">
            <input v-model="cfg.launchAtLogin" type="checkbox" @change="save" />
            <span>Launch at login</span>
          </label>
          <label v-if="isMac" class="check">
            <input v-model="cfg.showDockIcon" type="checkbox" @change="save" />
            <span>Show icon in the Dock<em>Off keeps Geda in the menu bar.</em></span>
          </label>
        </section>

        <section>
          <h2>Window</h2>
          <label class="check">
            <input v-model="cfg.previewOnHover" type="checkbox" @change="save" />
            <span>Show details when pointing at an entry<em>The card floats beside the popup.</em></span>
          </label>
          <div class="field">
            <span class="field-label">Open the popup at</span>
            <select v-model="cfg.popupPlacement" class="select" @change="save">
              <option value="cursor">The mouse pointer</option>
              <option value="menubar">{{ iconPlacementLabel }}</option>
            </select>
          </div>
          <div class="field">
            <span class="field-label">Popup size</span>
            <input v-model.number="cfg.popupWidth" class="num" type="number" min="300" max="1600" step="20" @change="onNumberChange('popupWidth')" />
            <span class="field-suffix">×</span>
            <input v-model.number="cfg.popupHeight" class="num" type="number" min="240" max="1200" step="20" @change="onNumberChange('popupHeight')" />
            <span class="field-suffix">px</span>
          </div>
        </section>

        <section>
          <button class="btn subtle" type="button" @click="resetDefaults">Reset all preferences</button>
        </section>
      </template>

      <template v-else-if="tab === 'clipboard' && cfg">
        <section>
          <h2>Behaviour</h2>
          <label v-if="pasteSupported" class="check">
            <input v-model="cfg.pasteOnSelect" type="checkbox" @change="save" />
            <span>Paste immediately when an entry is chosen<em>When off, choosing an entry only puts it on the clipboard.</em></span>
          </label>
          <p v-else class="hint">
            Choosing an entry copies it and returns you to the app you were working in,
            so it takes one {{ env?.modifierName ?? '⌘' }}V to paste.
          </p>
          <label class="check">
            <input v-model="cfg.captureImages" type="checkbox" @change="save" />
            <span>Record images as well as text</span>
          </label>
        </section>

        <section>
          <h2>History</h2>
          <div class="field">
            <span class="field-label">Keep at most</span>
            <input v-model.number="cfg.maxItems" class="num" type="number" min="10" max="2000" step="10" @change="onNumberChange('maxItems')" />
            <span class="field-suffix">entries</span>
          </div>
          <p class="hint">Pinned entries are always kept, regardless of this limit.</p>
        </section>

        <section v-if="pasteSupported && !canPaste">
          <h2>Permission needed</h2>
          <p class="hint warn">
            Pasting automatically requires Accessibility permission. Without it, choosing an
            entry still copies it to the clipboard and you paste it yourself.
          </p>
          <div class="btn-row">
            <button class="btn" type="button" @click="requestPastePermission">Grant Accessibility permission…</button>
            <button class="btn" type="button" @click="openPasteSettings">Open Accessibility settings…</button>
          </div>
        </section>
      </template>

      <template v-else-if="tab === 'privacy' && cfg">
        <section>
          <h2>Excluded copies</h2>
          <label class="check">
            <input v-model="cfg.ignoreConcealed" type="checkbox" @change="save" />
            <span>Skip entries marked confidential<em>This is how password managers ask to be excluded.</em></span>
          </label>
          <label class="check">
            <input v-model="cfg.ignoreTransient" type="checkbox" @change="save" />
            <span>Skip entries marked temporary by the source app</span>
          </label>
          <div class="field stacked">
            <span class="field-label">Never record copies from these apps</span>
            <textarea
              v-model="ignoredText"
              rows="5"
              placeholder="One application name per line, e.g.&#10;1Password&#10;Keychain Access"
              spellcheck="false"
              @change="onIgnoredInput"
            />
          </div>
        </section>

        <section>
          <h2>Local data</h2>
          <p class="hint">
            Clipboard content and statistics stay on this device. Statistics contain only
            hourly and daily counters — never content, hashes, or source applications — and
            are automatically limited to 370 days.
          </p>
          <div class="btn-row">
            <button class="btn danger" type="button" @click="clearHistoryAndStatistics">
              Clear history and statistics…
            </button>
          </div>
        </section>
      </template>

      <StatisticsView v-else-if="tab === 'statistics'" />

      <template v-else-if="tab === 'about'">
        <div class="about">
          <h1>Geda Clipboard</h1>
          <p class="version">Version {{ env?.version ?? '—' }}</p>
          <div v-if="canUpdate" class="btn-row about-actions">
            <button class="btn" type="button" @click="App.CheckForUpdates()">Check for Updates…</button>
          </div>
          <p class="blurb">
            A menu bar clipboard manager: it keeps what you copy, tells you when it does, and
            brings any earlier entry back to the app you were using.
          </p>
          <dl class="about-meta">
            <dt>Platform</dt>
            <dd>{{ env?.platform ?? '—' }}</dd>
            <dt>Automatic paste</dt>
            <dd v-if="!pasteSupported">Not in this build</dd>
            <dd v-else>{{ canPaste ? 'Permitted' : 'Needs Accessibility permission' }}</dd>
            <dt>Notifications</dt>
            <dd>{{ notifyStatus === 'authorized' ? 'Permitted' : notifyStatus === 'denied' ? 'Turned off' : notifyStatus === 'notDetermined' ? 'Not granted yet' : 'Unknown' }}</dd>
          </dl>
          <div class="about-links" aria-label="Legal and support links">
            <button type="button" @click="BrowserOpenURL(privacyURL)">Privacy Policy</button>
            <button type="button" @click="BrowserOpenURL(supportURL)">Support</button>
            <button type="button" @click="BrowserOpenURL(termsURL)">Terms</button>
          </div>
          <p class="credit">Inspired by <span class="mono">Maccy</span>, built with Go and Wails.</p>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.header {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 9px var(--pad-x);
}

.tabs {
  display: flex;
  gap: 2px;
  padding: 2px;
  border-radius: 7px;
  background: var(--field-bg);
}

.tab {
  padding: 4px 10px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  font-size: 12.5px;
  cursor: default;
}

.tab.active {
  background: var(--accent);
  color: var(--accent-fg);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status {
  font-size: 12px;
  color: var(--fg-dim);
}

.close {
  padding: 4px 12px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--field-bg);
  font-size: 12.5px;
  cursor: default;
}

.close:hover {
  background: var(--hover-bg);
}

.body {
  flex: 1;
  min-height: 0;
  padding: 4px 18px 20px;
}

section {
  padding: 14px 0;
  border-bottom: 1px solid var(--hairline);
}

section:last-child {
  border-bottom: 0;
}

h2 {
  margin: 0 0 10px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--fg-dim);
}

.check {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 4px 0;
}

.check input {
  margin: 1px 0 0;
  flex: none;
}

.check span {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.check em {
  font-style: normal;
  font-size: 11.5px;
  color: var(--fg-dim);
}

.field {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 0;
}

.field.stacked {
  flex-direction: column;
  align-items: stretch;
  gap: 6px;
}

.field-label {
  color: var(--fg);
}

.field-suffix {
  color: var(--fg-dim);
}

.num {
  width: 82px;
  padding: 4px 8px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--field-bg);
  outline: 0;
  user-select: text;
  -webkit-user-select: text;
}

.select {
  padding: 4px 8px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--field-bg);
  outline: 0;
}

textarea {
  width: 100%;
  padding: 7px 9px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--field-bg);
  outline: 0;
  resize: vertical;
  font-size: 12.5px;
  line-height: 1.5;
  user-select: text;
  -webkit-user-select: text;
}

.hotkey {
  min-width: 110px;
  padding: 4px 12px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--field-bg);
  font-size: 13px;
  cursor: default;
}

.hotkey.capturing {
  border-color: var(--accent);
  background: color-mix(in srgb, var(--accent) 22%, transparent);
}

.hint {
  margin: 6px 0 0;
  font-size: 11.5px;
  color: var(--fg-dim);
}

.hint.warn {
  color: var(--danger);
}

.btn-row {
  display: flex;
  gap: 8px;
  margin: 8px 0 4px;
}

.btn-row .btn {
  margin-top: 0;
}

.btn {
  margin-top: 10px;
  padding: 5px 13px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--field-bg);
  font-size: 12.5px;
  cursor: default;
}

.btn:hover {
  background: var(--hover-bg);
}

.btn.subtle {
  margin-top: 0;
  color: var(--fg-dim);
}

.btn.danger {
  color: var(--danger);
}

.about {
  padding: 22px 4px;
  text-align: center;
}

.about h1 {
  margin: 0;
  font-size: 21px;
  font-weight: 600;
}

.version {
  margin: 4px 0 0;
  font-size: 12.5px;
  color: var(--fg-dim);
}

.blurb {
  margin: 16px auto 0;
  max-width: 44ch;
  color: var(--fg-dim);
  line-height: 1.55;
}

.about-meta {
  display: grid;
  grid-template-columns: auto auto;
  justify-content: center;
  gap: 4px 14px;
  margin: 20px 0 0;
  font-size: 12.5px;
  text-align: left;
}

.about-meta dt {
  color: var(--fg-dim);
}

.about-meta dd {
  margin: 0;
}

.about-actions {
  justify-content: center;
}

.about-links {
  display: flex;
  justify-content: center;
  gap: 6px;
  margin-top: 18px;
}

.about-links button {
  padding: 3px 7px;
  border: 0;
  background: transparent;
  color: var(--accent);
  font-size: 11.5px;
}

.about-links button:hover {
  text-decoration: underline;
}

.credit {
  margin: 22px 0 0;
  font-size: 11.5px;
  color: var(--fg-faint);
}

.mono {
  font-family: var(--font-mono);
}
</style>
