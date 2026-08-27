<script lang="ts" setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import type { main } from '../../wailsjs/go/models'
import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { formatHotkey } from '../lib/keys'

const props = defineProps<{
  env: main.Environment | null
}>()

const primary = ref<HTMLButtonElement | null>(null)
const hotkey = ref('')
const busy = ref(false)

const isMac = computed(() => props.env?.platform !== 'windows')
const iconHome = computed(() => (isMac.value ? 'menu bar' : 'system tray'))

async function loadHotkey(): Promise<void> {
  try {
    hotkey.value = formatHotkey((await App.GetSettings()).hotkey)
  } catch {
    hotkey.value = isMac.value ? '⇧⌘V' : 'Ctrl+Shift+V'
  }
}

function focusPrimary(): void {
  nextTick(() => primary.value?.focus({ preventScroll: true }))
}

async function complete(openSettings: boolean): Promise<void> {
  if (busy.value) return
  busy.value = true
  try {
    await App.CompleteWelcome(openSettings)
  } finally {
    busy.value = false
  }
}

let disposeShown: (() => void) | undefined

onMounted(() => {
  void loadHotkey()
  focusPrimary()
  disposeShown = EventsOn('welcome:shown', focusPrimary)
})

onUnmounted(() => disposeShown?.())
</script>

<template>
  <main class="panel welcome" aria-labelledby="welcome-title">
    <div class="menu-strip" aria-hidden="true">
      <span class="menu-dot" />
      <span class="menu-line short" />
      <span class="menu-line" />
      <span class="tray-mark">
        <svg viewBox="0 0 20 20">
          <path d="M6.5 5.5h7v9h-7zM8 3.5h4v2H8z" />
          <path d="M8.5 8h3M8.5 10.5h3M8.5 13h2" />
        </svg>
      </span>
    </div>

    <section class="intro">
      <div class="app-mark" aria-hidden="true">
        <svg viewBox="0 0 32 32">
          <rect x="8" y="5" width="16" height="22" rx="4" />
          <path d="M12 11h8M12 15h8M12 19h5" />
          <path d="M13 5.5V4h6v1.5" />
        </svg>
      </div>
      <p class="eyebrow">Geda Clipboard</p>
      <h1 id="welcome-title">Geda is ready</h1>
      <p class="lede">
        Geda stays in your {{ iconHome }}, quietly saving what you copy without keeping a
        window in the way.
      </p>
    </section>

    <section class="how" aria-label="How to open Geda">
      <div class="how-item">
        <span class="step">1</span>
        <span class="how-copy">
          <strong>Look for the clipboard icon</strong>
          <small>It lives in the {{ iconHome }} whenever Geda is running.</small>
        </span>
      </div>
      <div class="how-item">
        <span class="step">2</span>
        <span class="how-copy">
          <strong>Open it from anywhere</strong>
          <small>Click the icon or press the global shortcut.</small>
        </span>
        <kbd>{{ hotkey || (isMac ? '⇧⌘V' : 'Ctrl+Shift+V') }}</kbd>
      </div>
    </section>

    <p v-if="isMac" class="manager-note">
      Using Bartender, Ice, or another menu bar manager? You may need to unhide Geda there.
    </p>

    <div class="actions">
      <button class="secondary" type="button" :disabled="busy" @click="complete(true)">
        Open Preferences
      </button>
      <button ref="primary" class="primary" type="button" :disabled="busy" @click="complete(false)">
        Start using Geda
      </button>
    </div>
  </main>
</template>

<style scoped>
.welcome {
  padding: 0 46px 34px;
}

.menu-strip {
  display: flex;
  align-items: center;
  align-self: stretch;
  height: 28px;
  margin: 0 -46px;
  padding: 0 12px;
  border-bottom: 1px solid var(--hairline);
  background: color-mix(in srgb, var(--field-bg) 55%, transparent);
}

.menu-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--fg-faint);
}

.menu-line {
  width: 36px;
  height: 4px;
  margin-left: 8px;
  border-radius: 2px;
  background: var(--fg-faint);
  opacity: 0.55;
}

.menu-line.short {
  width: 22px;
}

.tray-mark {
  display: grid;
  width: 24px;
  height: 21px;
  margin-left: auto;
  place-items: center;
  border-radius: 5px;
  background: var(--accent);
  color: var(--accent-fg);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 18%, transparent);
}

.tray-mark svg {
  width: 16px;
  height: 16px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.4;
}

.intro {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 28px;
  text-align: center;
}

.app-mark {
  display: grid;
  width: 52px;
  height: 52px;
  margin-bottom: 13px;
  place-items: center;
  border: 1px solid color-mix(in srgb, var(--accent) 60%, var(--panel-border));
  border-radius: 14px;
  background: linear-gradient(145deg, color-mix(in srgb, var(--accent) 86%, white), var(--accent));
  color: white;
  box-shadow: 0 10px 28px color-mix(in srgb, var(--accent) 28%, transparent);
}

.app-mark svg {
  width: 31px;
  height: 31px;
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.6;
}

.eyebrow {
  margin: 0 0 2px;
  color: var(--fg-dim);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

h1 {
  margin: 0;
  font-size: 25px;
  font-weight: 650;
  letter-spacing: -0.02em;
}

.lede {
  max-width: 48ch;
  margin: 8px 0 0;
  color: var(--fg-dim);
  line-height: 1.55;
}

.how {
  display: flex;
  flex-direction: column;
  gap: 7px;
  margin-top: 22px;
}

.how-item {
  display: flex;
  min-height: 49px;
  align-items: center;
  gap: 11px;
  padding: 8px 11px;
  border: 1px solid var(--panel-border);
  border-radius: 9px;
  background: color-mix(in srgb, var(--field-bg) 70%, transparent);
}

.step {
  display: grid;
  flex: none;
  width: 25px;
  height: 25px;
  place-items: center;
  border-radius: 50%;
  background: color-mix(in srgb, var(--accent) 20%, transparent);
  color: color-mix(in srgb, var(--accent) 75%, white);
  font-size: 11.5px;
  font-weight: 700;
}

.how-copy {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
}

.how-copy strong {
  font-size: 12.5px;
  font-weight: 600;
}

.how-copy small {
  margin-top: 1px;
  color: var(--fg-dim);
  font-size: 11.5px;
}

kbd {
  flex: none;
  padding: 5px 9px;
  border: 1px solid var(--panel-border);
  border-bottom-color: color-mix(in srgb, var(--panel-border) 45%, black);
  border-radius: 6px;
  background: var(--panel-bg);
  box-shadow: 0 2px 0 color-mix(in srgb, var(--panel-border) 70%, transparent);
  color: var(--fg);
  font-family: var(--font-ui);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.manager-note {
  margin: 12px 4px 0;
  color: var(--fg-faint);
  font-size: 11px;
  text-align: center;
}

.actions {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: auto;
  padding-top: 20px;
}

button {
  min-width: 126px;
  padding: 7px 14px;
  border-radius: 7px;
  font-weight: 550;
  cursor: default;
}

button:disabled {
  opacity: 0.48;
}

.secondary {
  border: 1px solid var(--panel-border);
  background: var(--field-bg);
}

.secondary:hover:not(:disabled) {
  background: var(--hover-bg);
}

.primary {
  border: 1px solid color-mix(in srgb, var(--accent) 70%, white);
  background: var(--accent);
  color: var(--accent-fg);
}

.primary:hover:not(:disabled) {
  filter: brightness(1.08);
}

button:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--accent) 70%, white);
  outline-offset: 2px;
}
</style>
