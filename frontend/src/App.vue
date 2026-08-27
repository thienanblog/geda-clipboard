<script lang="ts" setup>
/** Shell: switches between the popup list and preferences, and tells the Go
 *  side when the window loses focus so the popup can dismiss itself. */
import { onMounted, onUnmounted, ref } from 'vue'
import type { main } from '../wailsjs/go/models'
import * as App from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { setPlatform } from './lib/keys'
import PopupView from './components/PopupView.vue'
import SettingsView from './components/SettingsView.vue'
import WelcomeView from './components/WelcomeView.vue'

type View = 'popup' | 'settings' | 'welcome'
type SettingsTab = 'general' | 'clipboard' | 'pinned' | 'privacy' | 'statistics' | 'about'

const view = ref<View>('popup')
const settingsTab = ref<SettingsTab>('general')
const env = ref<main.Environment | null>(null)

function onBlur(): void {
  // The Go side decides whether a blur should dismiss: the popup hides, the
  // settings view stays open.
  void App.OnWindowBlur()
}

function openSettings(tab: SettingsTab): void {
  settingsTab.value = tab
  view.value = 'settings'
  void App.ShowSettings()
}

/** Closing preferences dismisses the window instead of falling back to the
 *  list. Reopening the popup is a tray click or hotkey away, and showing it
 *  unasked leaves a panel on screen the user did not ask for. The switch back
 *  to the list is left to the Go side, which makes it once the window is off
 *  screen; doing it here would flash the list on the way out. */
function closeSettings(): void {
  void App.HidePopup()
}

/** The window is bigger than the panel by the shadow margins, so there is a rim
 *  of transparent window around it. Clicking there dismisses the popup, the way
 *  clicking outside a menu does, rather than swallowing the click. Preferences
 *  are dismissed by their own controls, not by a stray click. */
function onSurroundClick(): void {
  if (view.value === 'popup') void App.HidePopup()
}

/** Mouse-down on the surround must not pull focus off the search field. */
function keepFocus(event: MouseEvent): void {
  event.preventDefault()
}

let disposers: Array<() => void> = []

onMounted(async () => {
  window.addEventListener('blur', onBlur)

  // Install the listener before asking Go for bootstrap state: a repeat launch
  // can arrive while the WebView is starting, and its requested view must not
  // be overwritten by a late initial response.
  disposers.push(
    EventsOn('view:changed', (next: string) => {
      if (next === 'popup' || next === 'settings' || next === 'welcome') {
        view.value = next
      }
    }),
  )

  try {
    const environment = await App.Env()
    env.value = environment
    setPlatform(environment.platform)
  } catch {
    /* Fall back to the default (macOS) key labels. */
  }

  try {
    const initial = await App.InitialView()
    if (initial === 'popup' || initial === 'settings' || initial === 'welcome') {
      view.value = initial
    }
  } catch {
    /* Popup is the safe fallback for builds with older bindings. */
  }

  void App.FrontendReady()
})

onUnmounted(() => {
  window.removeEventListener('blur', onBlur)
  disposers.forEach((dispose) => dispose())
  disposers = []
})
</script>

<template>
  <div class="window" @mousedown.self="keepFocus" @click.self="onSurroundClick">
    <PopupView v-if="view === 'popup'" @open-settings="openSettings" />
    <WelcomeView v-else-if="view === 'welcome'" :env="env" />
    <SettingsView
      v-else
      :tab="settingsTab"
      :env="env"
      @close="closeSettings"
      @tab="settingsTab = $event"
    />
  </div>
</template>

<style scoped>
/* The panel is inset from the window on every side, leaving a transparent rim
   for its own drop shadow to fall into. */
.window {
  height: 100%;
  padding: var(--shadow-top) var(--shadow-side) var(--shadow-bottom);
}
</style>
