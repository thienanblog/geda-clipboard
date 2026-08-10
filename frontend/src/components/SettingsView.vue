<script lang="ts" setup>
/** Preferences and About. Changes are saved as soon as they are made, which is
 *  what a menu bar utility should do -- there is no OK/Cancel. */
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { main, settings } from '../../wailsjs/go/models'
import * as App from '../../wailsjs/go/main/App'
import { eventToSpec, formatHotkey } from '../lib/keys'

const props = defineProps<{
  tab: 'general' | 'about'
  env: main.Environment | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'tab', value: 'general' | 'about'): void
}>()

const cfg = ref<settings.Settings | null>(null)
const status = ref('')
const capturingHotkey = ref(false)
const canPaste = ref(true)
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

async function load(): Promise<void> {
  cfg.value = await App.GetSettings()
  ignoredText.value = (cfg.value.ignoredApps ?? []).join('\n')
  canPaste.value = props.env?.canPaste ?? true
  try {
    notifyStatus.value = await App.NotificationStatus()
  } catch {
    notifyStatus.value = 'unknown'
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
  void save()
}

async function requestPastePermission(): Promise<void> {
  canPaste.value = await App.RequestPastePermission()
  if (!canPaste.value) {
    flash('Permission not granted yet — check System Settings')
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
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown, true)
  window.clearTimeout(statusTimer)
})
</script>

<template>
  <div class="panel">
    <header class="header">
      <nav class="tabs">
        <button
          class="tab"
          :class="{ active: tab === 'general' }"
          type="button"
          @click="emit('tab', 'general')"
        >
          General
        </button>
        <button
          class="tab"
          :class="{ active: tab === 'about' }"
          type="button"
          @click="emit('tab', 'about')"
        >
          About
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
            <span>
              Notify when something is copied
              <em>Shows the source app and a preview of the new entry.</em>
            </span>
          </label>
          <label class="check">
            <input v-model="cfg.notifyOnPaste" type="checkbox" @change="save" />
            <span>
              Notify when an entry is pasted
              <em>Confirms which app received the paste.</em>
            </span>
          </label>

          <div class="btn-row">
            <button class="btn" type="button" @click="sendTest">Send a test notification</button>
          </div>
        </section>

        <section>
          <h2>Behaviour</h2>
          <label class="check">
            <input v-model="cfg.pasteOnSelect" type="checkbox" @change="save" />
            <span>
              Paste immediately when an entry is chosen
              <em>When off, choosing an entry only puts it on the clipboard.</em>
            </span>
          </label>
          <label class="check">
            <input v-model="cfg.captureImages" type="checkbox" @change="save" />
            <span>
              Record images as well as text
            </span>
          </label>
          <label class="check">
            <input v-model="cfg.previewOnHover" type="checkbox" @change="save" />
            <span>
              Show details beside the list when pointing at an entry
              <em>The card floats to the left of the popup; it costs no width.</em>
            </span>
          </label>
          <label class="check">
            <input v-model="cfg.launchAtLogin" type="checkbox" @change="save" />
            <span>Launch at login</span>
          </label>
        </section>

        <section>
          <h2>Shortcut</h2>
          <div class="field">
            <span class="field-label">Toggle the popup</span>
            <button
              class="hotkey"
              :class="{ capturing: capturingHotkey }"
              type="button"
              @click="startHotkeyCapture"
            >
              {{ hotkeyLabel }}
            </button>
          </div>
          <p class="hint">Click the shortcut, then press the key combination you want.</p>
        </section>

        <section>
          <h2>History</h2>
          <div class="field">
            <span class="field-label">Keep at most</span>
            <input
              v-model.number="cfg.maxItems"
              class="num"
              type="number"
              min="10"
              max="2000"
              step="10"
              @change="onNumberChange('maxItems')"
            />
            <span class="field-suffix">entries</span>
          </div>
          <p class="hint">Pinned entries are always kept, regardless of this limit.</p>
        </section>

        <section>
          <h2>Privacy</h2>
          <label class="check">
            <input v-model="cfg.ignoreConcealed" type="checkbox" @change="save" />
            <span>
              Skip entries marked confidential
              <em>This is how password managers ask to be excluded.</em>
            </span>
          </label>
          <label class="check">
            <input v-model="cfg.ignoreTransient" type="checkbox" @change="save" />
            <span>Skip entries marked temporary by the source app</span>
          </label>

          <div class="field stacked">
            <span class="field-label">Never record copies from these apps</span>
            <textarea
              v-model="ignoredText"
              rows="4"
              placeholder="One application name per line, e.g.&#10;1Password&#10;Keychain Access"
              spellcheck="false"
              @change="onIgnoredInput"
            />
          </div>
        </section>

        <section v-if="!canPaste">
          <h2>Permission needed</h2>
          <p class="hint warn">
            Pasting automatically requires Accessibility permission. Without it, choosing an
            entry still copies it to the clipboard — you just have to paste yourself.
          </p>
          <button class="btn" type="button" @click="requestPastePermission">
            Grant Accessibility permission…
          </button>
        </section>

        <section>
          <h2>Window</h2>
          <div class="field">
            <span class="field-label">Open the popup at</span>
            <select v-model="cfg.popupPlacement" class="select" @change="save">
              <option value="cursor">The mouse pointer</option>
              <option value="menubar">{{ iconPlacementLabel }}</option>
            </select>
          </div>
          <p class="hint">
            At the pointer the popup opens wherever you are working, like a context menu.
          </p>

          <div class="field">
            <span class="field-label">Popup size</span>
            <input
              v-model.number="cfg.popupWidth"
              class="num"
              type="number"
              min="300"
              max="1600"
              step="20"
              @change="onNumberChange('popupWidth')"
            />
            <span class="field-suffix">×</span>
            <input
              v-model.number="cfg.popupHeight"
              class="num"
              type="number"
              min="240"
              max="1200"
              step="20"
              @change="onNumberChange('popupHeight')"
            />
            <span class="field-suffix">px</span>
          </div>
        </section>

        <section>
          <button class="btn subtle" type="button" @click="resetDefaults">
            Reset all preferences
          </button>
        </section>
      </template>

      <template v-else-if="tab === 'about'">
        <div class="about">
          <h1>Geda Clipboard</h1>
          <p class="version">Version {{ env?.version ?? '—' }}</p>
          <p class="blurb">
            A menu bar clipboard manager: it keeps what you copy, tells you when it does, and
            pastes any earlier entry straight back into the app you were using.
          </p>
          <dl class="about-meta">
            <dt>Platform</dt>
            <dd>{{ env?.platform ?? '—' }}</dd>
            <dt>Automatic paste</dt>
            <dd>{{ canPaste ? 'Permitted' : 'Needs Accessibility permission' }}</dd>
            <dt>Notifications</dt>
            <dd>{{ notifyStatus === 'authorized' ? 'Permitted' : notifyStatus === 'denied' ? 'Turned off' : notifyStatus === 'notDetermined' ? 'Not granted yet' : 'Unknown' }}</dd>
          </dl>
          <p class="credit">
            Inspired by <span class="mono">Maccy</span>, built with Go and Wails.
          </p>
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
  padding: 4px 14px;
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

.credit {
  margin: 22px 0 0;
  font-size: 11.5px;
  color: var(--fg-faint);
}

.mono {
  font-family: var(--font-mono);
}
</style>
