<script lang="ts" setup>
/** The menu bar popup: search field, clipboard history list with numeric
 *  accelerators, and the action footer. Details live in a card that flies out
 *  into the transparent gutter on the left, so the panel itself stays narrow. */
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import type { store } from '../../wailsjs/go/models'
import * as App from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { rowLabel } from '../lib/format'
import { combo, hasPrimary, sym } from '../lib/keys'
import DetailPane from './DetailPane.vue'

const emit = defineEmits<{
  /** Ask the shell to open preferences on a specific tab. */
  (event: 'open-settings', tab: 'general' | 'about'): void
}>()

const items = ref<store.Item[]>([])
const query = ref('')
const selected = ref(0)
const searchEl = ref<HTMLInputElement | null>(null)
const listEl = ref<HTMLElement | null>(null)
const rowEls = ref<HTMLElement[]>([])
const gutterEl = ref<HTMLElement | null>(null)
const flyoutEl = ref<HTMLElement | null>(null)

/** Index of the row under the cursor, or -1 when the mouse is away. */
const hovered = ref(-1)
/** Set while the user is driving the list from the keyboard, so the card can
 *  follow the selection with the mouse away. */
const keyboardNav = ref(false)
const previewOnHover = ref(true)

/** Live width of the list, kept by a ResizeObserver: the popup size is
 *  user-configurable, so the row text budget depends on it. */
const listWidth = ref(0)

/** Width of the transparent strip the window reserves on the left for the
 *  preview card, straight from the Go side; 0 when previews are off. */
const gutter = ref(0)

/** Distance from the top of the window to the preview card, in pixels. */
const flyoutTop = ref(8)

const errorMessage = ref('')
let errorTimer: number | undefined

const empty = computed(() => items.value.length === 0)

/** Hovering wins; with the mouse away the card follows keyboard selection. */
const detailIndex = computed(() => (hovered.value >= 0 ? hovered.value : selected.value))

const detailItem = computed<store.Item | null>(() => items.value[detailIndex.value] ?? null)

/** The card is a hover affordance: it stays out of the way until the user
 *  points at a row, or starts walking the list with the arrow keys. */
const showDetail = computed(
  () =>
    previewOnHover.value &&
    gutter.value > 0 &&
    detailItem.value !== null &&
    (hovered.value >= 0 || keyboardNav.value),
)

/** How many characters of a row's label actually fit. The reserved slice
 *  covers the row padding, the pin glyph and the ⌘n accelerator; 6.6px is the
 *  average advance of the 13px UI font. */
const labelBudget = computed(() => {
  if (listWidth.value === 0) return 60
  return Math.max(16, Math.min(160, Math.round((listWidth.value - 115) / 6.6)))
})

function label(item: store.Item): string {
  return rowLabel(item, labelBudget.value)
}

/** Only the first nine rows get a numeric accelerator, as in the reference. */
function accelerator(index: number): string {
  return index < 9 ? `${sym.cmd} ${index + 1}` : ''
}

async function reload(): Promise<void> {
  try {
    items.value = await App.List(query.value)
  } catch (err) {
    items.value = []
    showError(String(err))
    return
  }
  if (selected.value >= items.value.length) {
    selected.value = Math.max(0, items.value.length - 1)
  }
  // Rows are only ever assigned into this array, so a shorter list would leave
  // detached elements behind -- and the preview card, which places itself off
  // the row's position, would then read a rect of zeroes.
  rowEls.value.length = items.value.length
}

async function loadSettings(): Promise<void> {
  try {
    const cfg = await App.GetSettings()
    previewOnHover.value = cfg.previewOnHover
  } catch {
    /* Defaults are fine if preferences cannot be read. */
  }
  try {
    gutter.value = await App.PopupGutter()
  } catch {
    gutter.value = 0
  }
}

function showError(message: string): void {
  errorMessage.value = message
  window.clearTimeout(errorTimer)
  errorTimer = window.setTimeout(() => (errorMessage.value = ''), 4000)
}

function focusSearch(): void {
  nextTick(() => searchEl.value?.focus())
}

/** Keeps the selected row inside the scroll viewport during keyboard nav. */
function scrollSelectedIntoView(): void {
  nextTick(() => {
    rowEls.value[selected.value]?.scrollIntoView({ block: 'nearest' })
  })
}

/** Lines the preview card up with the row it describes, then keeps it inside
 *  the gutter: a row near the bottom would otherwise push the card off screen.
 *  The card is placed within the gutter, which is itself inset from the window
 *  by the shadow margins, so both ends of the sum are measured from there.
 *  Runs after the DOM settles, since the card's height depends on the entry. */
function positionFlyout(): void {
  nextTick(() => {
    const row = rowEls.value[detailIndex.value]
    const card = flyoutEl.value
    const gutterBox = gutterEl.value
    if (!row || !card || !gutterBox) return
    const margin = 8
    const top = row.getBoundingClientRect().top - gutterBox.getBoundingClientRect().top - 6
    const lowest = Math.max(margin, gutterBox.clientHeight - card.offsetHeight - margin)
    flyoutTop.value = Math.max(margin, Math.min(top, lowest))
  })
}

/** A card holding an image grows once the thumbnail decodes, well after the
 *  first placement, which would leave it hanging past the bottom of the
 *  window. Re-placing on its own resize covers that without polling. */
let cardObserver: ResizeObserver | undefined

watch(flyoutEl, (card) => {
  cardObserver?.disconnect()
  if (!card) return
  cardObserver = new ResizeObserver(() => positionFlyout())
  cardObserver.observe(card)
})

function move(delta: number): void {
  if (empty.value) return
  const count = items.value.length
  keyboardNav.value = true
  selected.value = (selected.value + delta + count) % count
  scrollSelectedIntoView()
}

async function activate(index: number): Promise<void> {
  const item = items.value[index]
  if (!item) return
  try {
    await App.Select(item.id)
  } catch (err) {
    showError(String(err))
  }
}

async function copyOnly(index: number): Promise<void> {
  const item = items.value[index]
  if (!item) return
  try {
    await App.CopyOnly(item.id)
  } catch (err) {
    showError(String(err))
  }
}

async function togglePin(index: number): Promise<void> {
  const item = items.value[index]
  if (!item) return
  try {
    await App.TogglePin(item.id)
    await reload()
  } catch (err) {
    showError(String(err))
  }
}

async function remove(index: number): Promise<void> {
  const item = items.value[index]
  if (!item) return
  try {
    await App.Delete(item.id)
    await reload()
  } catch (err) {
    showError(String(err))
  }
}

async function clearAll(): Promise<void> {
  try {
    await App.ClearAll()
    await reload()
  } catch (err) {
    showError(String(err))
  }
}

function onRowEnter(index: number): void {
  selected.value = index
  keyboardNav.value = false
  if (previewOnHover.value) hovered.value = index
}

function onRowLeave(index: number): void {
  if (hovered.value === index) hovered.value = -1
}

function onListLeave(): void {
  hovered.value = -1
}

/** Mouse-down anywhere in the popup must not steal focus from the search
 *  field, otherwise typing stops working after the first click. */
function keepSearchFocus(event: MouseEvent): void {
  event.preventDefault()
}

/** The search field keeps focus the whole time the popup is open, so chords
 *  that mean something to a text field have to be handed back to it -- but only
 *  when there is actually text to act on. With an empty field ⌥⌫ has no word to
 *  delete and ⌘C has nothing to copy, which is when they should reach the list
 *  instead. Without this, ⌥⌫ silently deletes a history entry while the user is
 *  trying to correct their query. */
function searchHasText(): boolean {
  return searchEl.value === document.activeElement && query.value !== ''
}

function searchHasSelection(): boolean {
  const el = searchEl.value
  if (el !== document.activeElement || !el) return false
  return el.selectionStart !== el.selectionEnd
}

function onKeydown(event: KeyboardEvent): void {
  // Numeric accelerators: ⌘1..⌘9 / Ctrl+1..9.
  if (hasPrimary(event) && /^Digit[1-9]$/.test(event.code)) {
    event.preventDefault()
    const index = Number(event.code.slice(5)) - 1
    if (index < items.value.length) {
      selected.value = index
      void activate(index)
    }
    return
  }

  // ⌘, opens preferences.
  if (hasPrimary(event) && event.key === ',') {
    event.preventDefault()
    emit('open-settings', 'general')
    return
  }

  // ⌘Q quits.
  if (hasPrimary(event) && event.code === 'KeyQ') {
    event.preventDefault()
    void App.Quit()
    return
  }

  // ⌥⇧⌘⌫ clears the whole history.
  if (
    hasPrimary(event) &&
    event.altKey &&
    event.shiftKey &&
    (event.code === 'Backspace' || event.code === 'Delete')
  ) {
    event.preventDefault()
    void clearAll()
    return
  }

  // ⌥⌫ deletes the selected entry, unless the user is deleting a word of their
  // search query.
  if (event.altKey && (event.code === 'Backspace' || event.code === 'Delete')) {
    if (searchHasText()) return
    event.preventDefault()
    void remove(selected.value)
    return
  }

  // ⌥P pins or unpins the selected entry.
  if (event.altKey && event.code === 'KeyP') {
    event.preventDefault()
    void togglePin(selected.value)
    return
  }

  // ⌘C copies without pasting, unless the user is copying selected search text.
  if (hasPrimary(event) && event.code === 'KeyC') {
    if (searchHasSelection()) return
    event.preventDefault()
    void copyOnly(selected.value)
    return
  }

  switch (event.key) {
    case 'ArrowDown':
      event.preventDefault()
      move(1)
      break
    case 'ArrowUp':
      event.preventDefault()
      move(-1)
      break
    case 'Home':
      event.preventDefault()
      keyboardNav.value = true
      selected.value = 0
      scrollSelectedIntoView()
      break
    case 'End':
      event.preventDefault()
      keyboardNav.value = true
      selected.value = Math.max(0, items.value.length - 1)
      scrollSelectedIntoView()
      break
    case 'Enter':
      event.preventDefault()
      void activate(selected.value)
      break
    case 'Escape':
      event.preventDefault()
      // Escape clears an active search first, then closes the popup.
      if (query.value !== '') {
        query.value = ''
      } else {
        void App.HidePopup()
      }
      break
  }
}

watch(query, () => {
  selected.value = 0
  hovered.value = -1
  keyboardNav.value = false
  void reload()
})

// The card is anchored to a row, so it has to be re-placed whenever the entry
// it describes changes -- watching the item rather than the index also catches
// a reload swapping the entry out from under the same row.
watch([detailItem, showDetail], () => {
  if (showDetail.value) positionFlyout()
})

let disposers: Array<() => void> = []
let sizeObserver: ResizeObserver | undefined

function measure(): void {
  listWidth.value = listEl.value?.clientWidth ?? 0
}

onMounted(() => {
  void reload()
  void loadSettings()
  focusSearch()

  measure()
  sizeObserver = new ResizeObserver(measure)
  if (listEl.value) sizeObserver.observe(listEl.value)

  window.addEventListener('keydown', onKeydown)

  disposers.push(
    EventsOn('history:changed', () => {
      void reload()
    }),
  )
  disposers.push(
    EventsOn('view:changed', (view: string) => {
      if (view === 'popup') {
        // Reopening resets the search and returns to the newest entry.
        query.value = ''
        selected.value = 0
        hovered.value = -1
        keyboardNav.value = false
        void reload()
        void loadSettings()
        focusSearch()
      }
    }),
  )
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  window.clearTimeout(errorTimer)
  sizeObserver?.disconnect()
  cardObserver?.disconnect()
  disposers.forEach((dispose) => dispose())
  disposers = []
})
</script>

<template>
  <div class="stage">
    <!-- Transparent gutter: nothing is drawn here until a row is pointed at,
         which is what keeps the panel itself narrow. Clicking it dismisses the
         popup, the way clicking outside a menu does. -->
    <div
      v-if="gutter > 0"
      ref="gutterEl"
      class="gutter"
      :style="{ width: gutter + 'px' }"
      @mousedown="keepSearchFocus"
      @click="App.HidePopup()"
    >
      <transition name="flyout">
        <div
          v-if="showDetail"
          ref="flyoutEl"
          class="flyout scroll"
          :style="{ top: flyoutTop + 'px' }"
          @click.stop
        >
          <DetailPane :item="detailItem" />
        </div>
      </transition>
    </div>

    <div class="panel">
      <header class="header">
        <span class="brand">Geda</span>
        <div class="search">
          <svg class="search-icon" viewBox="0 0 16 16" aria-hidden="true">
            <path
              d="M6.5 1a5.5 5.5 0 0 1 4.38 8.84l3.64 3.64a.75.75 0 0 1-1.06 1.06l-3.64-3.64A5.5 5.5 0 1 1 6.5 1Zm0 1.5a4 4 0 1 0 0 8 4 4 0 0 0 0-8Z"
              fill="currentColor"
            />
          </svg>
          <input
            ref="searchEl"
            v-model="query"
            type="text"
            placeholder="type to search…"
            spellcheck="false"
            autocomplete="off"
          />
        </div>
      </header>

      <div class="hairline" />

      <div ref="listEl" class="list scroll" @mouseleave="onListLeave">
        <p v-if="empty" class="empty">
          {{ query ? 'No matching entries' : 'Clipboard history is empty' }}
        </p>

        <button
          v-for="(item, index) in items"
          :key="item.id"
          :ref="(el) => { if (el) rowEls[index] = el as HTMLElement }"
          class="row"
          :class="{ 'row-selected': index === selected }"
          type="button"
          @mousedown="keepSearchFocus"
          @click="activate(index)"
          @mouseenter="onRowEnter(index)"
          @mouseleave="onRowLeave(index)"
        >
          <svg v-if="item.pinned" class="pin" viewBox="0 0 16 16" aria-hidden="true">
            <path
              d="M9.5 1.5 14.5 6.5l-1.9.5-3 3 .4 3.3-1.3.5-2.2-3.4-3.6 3.6-.7-.7 3.6-3.6L2.4 7.5l.5-1.3 3.3.4 3-3 .3-2.1Z"
              fill="currentColor"
            />
          </svg>

          <img v-if="item.kind === 'image' && item.thumb" :src="item.thumb" class="thumb" alt="" />
          <span v-else class="label">{{ label(item) }}</span>

          <span class="accel">{{ accelerator(index) }}</span>
        </button>
      </div>

      <div class="hairline" />

      <footer class="footer">
        <button class="action" type="button" @click="clearAll">
          <span>Clear all</span>
          <span class="accel">{{ combo(sym.alt, sym.shift, sym.cmd, sym.del) }}</span>
        </button>
        <button class="action" type="button" @click="emit('open-settings', 'general')">
          <span>Preferences…</span>
          <span class="accel">{{ combo(sym.cmd, ',') }}</span>
        </button>
        <button class="action" type="button" @click="emit('open-settings', 'about')">
          <span>About</span>
        </button>
        <button class="action" type="button" @click="App.Quit()">
          <span>Quit</span>
          <span class="accel">{{ combo(sym.cmd, 'Q') }}</span>
        </button>
      </footer>

      <transition name="fade">
        <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
      </transition>
    </div>
  </div>
</template>

<style scoped>
/* The window is wider than the popup looks: the panel sits flush against its
   right edge, and the strip to the left is transparent until a preview flies
   out into it. */
.stage {
  display: flex;
  height: 100%;
}

.gutter {
  flex: none;
  position: relative;
}

.flyout {
  position: absolute;
  left: 0;
  right: 10px;
  max-height: calc(100% - 16px);
  background: var(--card-bg);
  border: 1px solid var(--panel-border);
  border-radius: var(--radius-panel);
  box-shadow: var(--panel-shadow);
}

.flyout-enter-active,
.flyout-leave-active {
  transition: opacity 0.12s ease, transform 0.12s ease;
}

.flyout-enter-from,
.flyout-leave-to {
  opacity: 0;
  /* Slides out of the panel it belongs to. */
  transform: translateX(8px);
}

.panel {
  flex: 1;
  min-width: 0;
}

.header {
  flex: none;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px var(--pad-x);
}

.brand {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
  flex: none;
}

.search {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 9px;
  border-radius: 6px;
  background: var(--field-bg);
}

.search-icon {
  width: 13px;
  height: 13px;
  flex: none;
  color: var(--fg-dim);
}

.search input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  font-size: 13px;
  /* The search field is the one place text selection makes sense. */
  user-select: text;
  -webkit-user-select: text;
}

.search input::placeholder {
  color: var(--fg-faint);
}

.list {
  flex: 1;
  min-height: 0;
  padding: 5px 6px;
}

.empty {
  margin: 0;
  padding: 24px 8px;
  text-align: center;
  color: var(--fg-faint);
  font-size: 12.5px;
}

.row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: var(--row-h);
  padding: 3px 8px;
  border: 0;
  border-radius: var(--radius-row);
  background: transparent;
  text-align: left;
  cursor: default;
}

.row:hover {
  background: var(--hover-bg);
}

.row-selected,
.row-selected:hover {
  background: var(--accent);
  color: var(--accent-fg);
}

.pin {
  width: 11px;
  height: 11px;
  flex: none;
  opacity: 0.8;
}

.label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.thumb {
  flex: 1;
  min-width: 0;
  max-height: 74px;
  border-radius: 3px;
  object-fit: contain;
  object-position: left center;
}

.footer {
  flex: none;
  display: flex;
  flex-direction: column;
  padding: 5px 6px;
}

.action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  padding: 4px 8px;
  border: 0;
  border-radius: var(--radius-row);
  background: transparent;
  text-align: left;
  cursor: default;
}

.action:hover {
  background: var(--accent);
  color: var(--accent-fg);
}

.action:hover .accel {
  color: rgba(255, 255, 255, 0.75);
}

.error {
  position: absolute;
  left: 12px;
  right: 12px;
  bottom: 12px;
  margin: 0;
  padding: 7px 10px;
  border-radius: 6px;
  background: var(--danger);
  color: #fff;
  font-size: 12px;
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.35);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
