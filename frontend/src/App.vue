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

type View = 'popup' | 'settings'

const view = ref<View>('popup')
const settingsTab = ref<'general' | 'about'>('general')
const env = ref<main.Environment | null>(null)

function onBlur(): void {
  // The Go side decides whether a blur should dismiss: the popup hides, the
  // settings view stays open.
  void App.OnWindowBlur()
}

function openSettings(tab: 'general' | 'about'): void {
  settingsTab.value = tab
  view.value = 'settings'
  void App.ShowSettings()
}

function closeSettings(): void {
  view.value = 'popup'
  void App.ShowPopupView()
}

let disposers: Array<() => void> = []

onMounted(async () => {
  try {
    const environment = await App.Env()
    env.value = environment
    setPlatform(environment.platform)
  } catch {
    /* Fall back to the default (macOS) key labels. */
  }

  window.addEventListener('blur', onBlur)

  disposers.push(
    EventsOn('view:changed', (next: string) => {
      if (next === 'popup' || next === 'settings') {
        view.value = next
      }
    }),
  )
})

onUnmounted(() => {
  window.removeEventListener('blur', onBlur)
  disposers.forEach((dispose) => dispose())
  disposers = []
})
</script>

<template>
  <PopupView v-if="view === 'popup'" @open-settings="openSettings" />
  <SettingsView
    v-else
    :tab="settingsTab"
    :env="env"
    @close="closeSettings"
    @tab="settingsTab = $event"
  />
</template>
