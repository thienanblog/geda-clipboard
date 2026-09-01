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
const gutterEl = ref<HTMLElement | null>(null)
const flyoutEl = ref<HTMLElement | null>(null)
const rowEls = new Map<number, HTMLElement>()

/** Index of the row under the cursor, or -1 when the mouse is away. */
const hovered = ref(-1)
/** Set while the user is driving the list from the keyboard, so the card can
 *  follow the selection with the mouse away. */
const keyboardNav = ref(false)
/** True until the keyboard has claimed the list. A freshly opened popup, or a
 *  new search, already highlights the first row without the user having walked
 *  to it, so the first Down has to land on that row instead of stepping past
 *  it -- otherwise opening the popup and pressing Down skips the newest entry. */
const primed = ref(true)
const previewOnHover = ref(true)
const imagePreviewSize = ref<'compact' | 'comfortable' | 'large'>('comfortable')
const clearPinnedOnHistoryClear = ref(false)
const scrollTop = ref(0)
const viewportHeight = ref(0)

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

const detailSummary = computed<store.Item | null>(() => items.value[detailIndex.value] ?? null)
const loadedDetail = ref<store.Item | null>(null)

/** The card is a hover affordance: it stays out of the way until the user
 *  points at a row, or starts walking the list with the arrow keys. */
const showDetail = computed(
  () =>
    previewOnHover.value &&
    gutter.value > 0 &&
    detailSummary.value !== null &&
    (hovered.value >= 0 || keyboardNav.value),
)

const detailItem = computed(() => loadedDetail.value ?? detailSummary.value)

/** The WebView only needs thumbnails for visible image rows and full text for
 *  the one detail card being inspected. A bounded image LRU prevents a long
 *  scroll session from rebuilding the old all-thumbnail memory footprint;
 *  text is deliberately not cached because one clipboard entry can be huge. */
const itemCacheLimit = 32
const itemCache = new Map<string, store.Item>()
const itemRequests = new Map<string, Promise<store.Item>>()
const itemCacheVersion = ref(0)
let cacheGeneration = 0

function clearItemCache(): void {
  cacheGeneration++
  itemCache.clear()
  itemRequests.clear()
  itemCacheVersion.value++
  loadedDetail.value = null
}

function cachedItem(id: string): store.Item | undefined {
  // Map is intentionally non-reactive; this scalar invalidates only consumers
  // of cached display data instead of proxying large base64 thumbnail strings.
  void itemCacheVersion.value
  return itemCache.get(id)
}

async function loadItem(id: string): Promise<store.Item> {
  const cached = itemCache.get(id)
  if (cached) {
    itemCache.delete(id)
    itemCache.set(id, cached)
    return cached
  }
  const pending = itemRequests.get(id)
  if (pending) return pending

  const generation = cacheGeneration
  const request = App.GetItem(id).then((item) => {
    if (generation !== cacheGeneration) return item
    if (item.kind === 'image') {
      itemCache.set(id, item)
      while (itemCache.size > itemCacheLimit) {
        const oldest = itemCache.keys().next().value as string | undefined
        if (!oldest) break
        itemCache.delete(oldest)
      }
      itemCacheVersion.value++
    }
    return item
  })
  itemRequests.set(id, request)
  const removePending = () => {
    if (itemRequests.get(id) === request) itemRequests.delete(id)
  }
  void request.then(removePending, removePending)
  return request
}

function thumbnail(item: store.Item): string {
  return cachedItem(item.id)?.thumb ?? ''
}

interface VirtualRow {
  item: store.Item
  index: number
  top: number
  height: number
}

// These values mirror --row-h and the image thumbnail heights plus the row's
// 3px vertical padding on each side. Keeping the geometry explicit makes
// keyboard scrolling deterministic even when the target row is not mounted.
function rowHeight(item: store.Item): number {
  if (item.kind !== 'image') return 30
  if (imagePreviewSize.value === 'compact') return 50
  if (imagePreviewSize.value === 'large') return 118
  return 80
}

const rowLayout = computed(() => {
  const offsets: number[] = []
  const heights: number[] = []
  let total = 0
  for (const item of items.value) {
    offsets.push(total)
    const height = rowHeight(item)
    heights.push(height)
    total += height
  }
  return { offsets, heights, total }
})

const visibleRows = computed<VirtualRow[]>(() => {
  const { offsets, heights } = rowLayout.value
  const overscan = 240
  const firstPixel = Math.max(0, scrollTop.value - overscan)
  const lastPixel = scrollTop.value + viewportHeight.value + overscan
  let start = 0
  while (start < items.value.length && offsets[start] + heights[start] < firstPixel) start++
  let end = start
  while (end < items.value.length && offsets[end] < lastPixel) end++
  return items.value.slice(start, end).map((item, relativeIndex) => {
    const index = start + relativeIndex
    return { item, index, top: offsets[index], height: heights[index] }
  })
})

/** How many characters of a row's label actually fit. The reserved slice
 *  covers the row padding and the shared trailing slot for the pin or ⌘n
 *  accelerator; 6.6px is the average advance of the 13px UI font. */
const labelBudget = computed(() => {
  if (listWidth.value === 0) return 60
  return Math.max(16, Math.min(160, Math.round((listWidth.value - 85) / 6.6)))
})

function label(item: store.Item): string {
  return rowLabel(item, labelBudget.value)
}

/** Only the first nine rows get a numeric accelerator, as in the reference. */
function accelerator(index: number): string {
  return index < 9 ? `${sym.cmd} ${index + 1}` : ''
}

/** What a screen reader reads for a row.
 *
 *  Not the visible label: that one is truncated to whatever fits the panel,
 *  and an image row has no text in it at all -- the cell is a thumbnail with
 *  an empty alt, so without this the row announces as nothing but its number.
 *  The pinned state is spoken too, since the pin is a glyph and aria-selected
 *  covers only the highlight. */
function rowDescription(item: store.Item): string {
  const parts: string[] = []
  if (item.pinned) parts.push('Pinned')
  parts.push(
    item.kind === 'image'
      ? `Image, ${item.imageW} by ${item.imageH} pixels`
      : rowLabel(item, 120),
  )
  if (item.sourceApp) parts.push(`from ${item.sourceApp}`)
  return parts.join(', ')
}

let listRequest = 0

async function reload(): Promise<void> {
  const request = ++listRequest
  try {
    const nextItems = await App.List(query.value)
    if (request !== listRequest) return
    items.value = nextItems
  } catch (err) {
    if (request !== listRequest) return
    items.value = []
    showError(String(err))
    return
  }
  if (selected.value >= items.value.length) {
    selected.value = Math.max(0, items.value.length - 1)
  }
}

async function loadSettings(): Promise<void> {
  try {
    const cfg = await App.GetSettings()
    previewOnHover.value = cfg.previewOnHover
    imagePreviewSize.value = ['compact', 'large'].includes(cfg.imagePreviewSize)
      ? cfg.imagePreviewSize as 'compact' | 'large'
      : 'comfortable'
    clearPinnedOnHistoryClear.value = cfg.clearPinnedOnHistoryClear
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

/** Puts the caret in the search field. The window is shown and focused by the
 *  Go side after the view is switched, and the web view hands focus back to the
 *  document while that is settling, so the claim is repeated on the next frame
 *  -- without it the popup opens with the caret nowhere and typing is lost. */
function focusSearch(): void {
  const claim = () => searchEl.value?.focus({ preventScroll: true })
  nextTick(() => {
    claim()
    requestAnimationFrame(claim)
  })
}

/** State a newly shown popup starts from: no search, newest entry highlighted,
 *  and the list not yet claimed by the keyboard. */
function resetForOpen(): void {
  query.value = ''
  selected.value = 0
  hovered.value = -1
  keyboardNav.value = false
  primed.value = true
  nextTick(() => {
    if (listEl.value) listEl.value.scrollTop = 0
    scrollTop.value = 0
  })
}

/** Keeps the selected row inside the scroll viewport during keyboard nav. */
function scrollSelectedIntoView(): void {
  const list = listEl.value
  const top = rowLayout.value.offsets[selected.value]
  const height = rowLayout.value.heights[selected.value]
  if (!list || top === undefined || height === undefined) return
  const bottom = top + height
  if (top < list.scrollTop) {
    list.scrollTop = top
  } else if (bottom > list.scrollTop + list.clientHeight) {
    list.scrollTop = bottom - list.clientHeight
  }
  scrollTop.value = list.scrollTop
  nextTick(positionFlyout)
}

function setRowElement(index: number, element: HTMLElement | null): void {
  if (element) {
    rowEls.set(index, element)
  } else {
    rowEls.delete(index)
  }
}

/** Lines the preview card up with the row it describes, then keeps it inside
 *  the gutter: a row near the bottom would otherwise push the card off screen.
 *  The card is placed within the gutter, which is itself inset from the window
 *  by the shadow margins, so both ends of the sum are measured from there.
 *  Runs after the DOM settles, since the card's height depends on the entry. */
function positionFlyout(): void {
  nextTick(() => {
    const row = rowEls.get(detailIndex.value)
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

let detailTimer: number | undefined
let detailRequest = 0

watch([detailSummary, showDetail], ([summary, visible]) => {
  window.clearTimeout(detailTimer)
  const request = ++detailRequest
  if (!summary || !visible) {
    loadedDetail.value = null
    return
  }

  loadedDetail.value = cachedItem(summary.id) ?? summary
  // A short dwell avoids bridge calls for rows merely crossed on the way to a
  // target. Visible image rows are already loading immediately below.
  detailTimer = window.setTimeout(() => {
    void loadItem(summary.id)
      .then((item) => {
        if (request === detailRequest && detailSummary.value?.id === item.id) {
          loadedDetail.value = item
        }
      })
      .catch(() => {
        // A concurrent delete can invalidate the row before its detail arrives.
      })
  }, 60)
})

watch(
  visibleRows,
  (rows) => {
    for (const row of rows) {
      if (row.item.kind === 'image') {
        void loadItem(row.item.id).catch(() => {
          // Missing entries disappear on the next history refresh.
        })
      }
    }
  },
  { immediate: true },
)

function move(delta: number): void {
  if (empty.value) return
  const count = items.value.length
  const claiming = primed.value && delta > 0
  keyboardNav.value = true
  primed.value = false
  // The first Down only claims the highlight that is already on the first row.
  if (!claiming) selected.value = (selected.value + delta + count) % count
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
    const nextIndex = items.value.findIndex((candidate) => candidate.id === item.id)
    if (nextIndex >= 0) selected.value = nextIndex
    scrollSelectedIntoView()
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
  const message = clearPinnedOnHistoryClear.value
    ? 'Clear all clipboard history, including pinned entries? This cannot be undone.'
    : 'Clear unpinned clipboard history? Pinned entries will be kept.'
  if (!window.confirm(message)) return
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

  // Enter belongs to a button reached with Tab. The global list handler only
  // owns Enter while the search field (or another non-button surface) keeps
  // focus; otherwise a keyboard user trying to pin would paste the row instead.
  if (event.key === 'Enter' && event.target instanceof HTMLButtonElement) return

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
      primed.value = false
      selected.value = 0
      scrollSelectedIntoView()
      break
    case 'End':
      event.preventDefault()
      keyboardNav.value = true
      primed.value = false
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
  primed.value = true
  if (listEl.value) listEl.value.scrollTop = 0
  scrollTop.value = 0
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
  viewportHeight.value = listEl.value?.clientHeight ?? 0
}

function onScroll(): void {
  scrollTop.value = listEl.value?.scrollTop ?? 0
}

onMounted(() => {
  void reload()
  void loadSettings()
  focusSearch()

  measure()
  sizeObserver = new ResizeObserver(measure)
  if (listEl.value) sizeObserver.observe(listEl.value)

  window.addEventListener('keydown', onKeydown)
  // The window can regain focus without a fresh show -- Windows hands focus
  // from the native frame into the web view after the popup appears -- and the
  // search field has to get the caret back each time.
  window.addEventListener('focus', focusSearch)

  disposers.push(
    EventsOn('history:changed', () => {
      clearItemCache()
      void reload()
    }),
  )
  disposers.push(
    EventsOn('view:changed', (view: string) => {
      if (view === 'popup') {
        // Reopening resets the search and returns to the newest entry. This
        // arrives before the window is on screen, so the caret is claimed from
        // "popup:shown" instead.
        resetForOpen()
        void reload()
        void loadSettings()
      }
    }),
  )
  disposers.push(EventsOn('popup:shown', () => focusSearch()))
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('focus', focusSearch)
  window.clearTimeout(errorTimer)
  window.clearTimeout(detailTimer)
  sizeObserver?.disconnect()
  cardObserver?.disconnect()
  disposers.forEach((dispose) => dispose())
  disposers = []
})
</script>

<template>
  <div class="stage" :class="`preview-${imagePreviewSize}`">
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
          <!-- The field keeps focus while arrow keys drive the list. The active
               descendant points at the primary row action; pinning remains a
               separate, valid button beside it rather than a nested control. -->
          <input
            ref="searchEl"
            v-model="query"
            type="text"
            placeholder="type to search…"
            spellcheck="false"
            autocomplete="off"
            role="searchbox"
            aria-label="Search clipboard history"
            aria-controls="geda-history"
            :aria-activedescendant="empty ? undefined : `geda-row-${selected}`"
          />
        </div>
      </header>

      <div class="hairline" />

      <div
        id="geda-history"
        ref="listEl"
        class="list scroll"
        role="list"
        aria-label="Clipboard history"
        @scroll="onScroll"
        @mouseleave="onListLeave"
      >
        <p v-if="empty" class="empty" role="status">
          {{ query ? 'No matching entries' : 'Clipboard history is empty' }}
        </p>

        <div
          v-else
          class="virtual-list"
          :style="{ height: rowLayout.total + 'px' }"
        >
          <div
            v-for="row in visibleRows"
            :key="row.item.id"
            :ref="(el) => setRowElement(row.index, el as HTMLElement | null)"
            class="row-shell"
            :class="{ 'row-selected': row.index === selected }"
            :style="{ height: row.height + 'px', transform: `translateY(${row.top}px)` }"
            role="listitem"
            :aria-posinset="row.index + 1"
            :aria-setsize="items.length"
            @mouseenter="onRowEnter(row.index)"
          >
            <button
              :id="`geda-row-${row.index}`"
              class="row-main"
              type="button"
              :aria-current="row.index === selected ? 'true' : undefined"
              :aria-label="rowDescription(row.item)"
              tabindex="-1"
              @mousedown="keepSearchFocus"
              @click="activate(row.index)"
            >
              <img
                v-if="row.item.kind === 'image' && thumbnail(row.item)"
                :src="thumbnail(row.item)"
                class="thumb"
                alt=""
                loading="lazy"
                decoding="async"
              />
              <span v-else-if="row.item.kind === 'image'" class="image-placeholder">Image</span>
              <span v-else class="label">{{ label(row.item) }}</span>
            </button>

            <!-- The accelerator and pin occupy one fixed slot so revealing the
                 mouse action never shifts or shortens the row content. A pinned
                 entry keeps its state visible at rest; an unpinned entry shows
                 its numeric accelerator until the pointer reaches the row. -->
            <div class="row-trailing" :class="{ pinned: row.item.pinned }">
              <span class="row-accel accel" aria-hidden="true">{{ accelerator(row.index) }}</span>
              <button
                class="pin-action"
                type="button"
                :title="row.item.pinned ? 'Unpin entry' : 'Pin entry'"
                :aria-label="`${row.item.pinned ? 'Unpin' : 'Pin'} ${rowDescription(row.item)}`"
                @mousedown="keepSearchFocus"
                @keydown.enter.stop.prevent="togglePin(row.index)"
                @click.stop="togglePin(row.index)"
              >
                <svg viewBox="0 0 16 16" aria-hidden="true">
                  <path
                    d="M9.5 1.5 14.5 6.5l-1.9.5-3 3 .4 3.3-1.3.5-2.2-3.4-3.6 3.6-.7-.7 3.6-3.6L2.4 7.5l.5-1.3 3.3.4 3-3 .3-2.1Z"
                    fill="currentColor"
                  />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="hairline" />

      <footer class="footer">
        <button class="action" type="button" @click="clearAll">
          <span>Clear history</span>
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
  --image-row-h: 74px;
  --image-detail-h: 170px;
  display: flex;
  height: 100%;
}

.stage.preview-compact {
  --image-row-h: 44px;
  --image-detail-h: 120px;
}

.stage.preview-large {
  --image-row-h: 112px;
  --image-detail-h: 220px;
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

.virtual-list {
  position: relative;
  width: 100%;
}

.row-shell {
  position: absolute;
  inset: 0 0 auto;
  display: flex;
  align-items: center;
  width: 100%;
  min-height: var(--row-h);
  border-radius: var(--radius-row);
  background: transparent;
  contain: layout paint style;
}

.row-main {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  gap: 8px;
  height: 100%;
  min-height: var(--row-h);
  padding: 3px 5px 3px 8px;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: default;
}

.row-shell:hover {
  background: var(--hover-bg);
}

.row-selected,
.row-selected:hover {
  background: var(--accent);
  color: var(--accent-fg);
}

.row-trailing {
  position: relative;
  width: 27px;
  height: 27px;
  margin-right: 3px;
  flex: none;
}

.row-accel,
.pin-action {
  position: absolute;
  inset: 0;
  transition: opacity 0.1s ease;
}

.row-accel {
  display: flex;
  align-items: center;
  justify-content: center;
}

.pin-action {
  display: grid;
  width: 27px;
  height: 27px;
  padding: 6px;
  place-items: center;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: inherit;
  opacity: 0;
  pointer-events: none;
  cursor: default;
}

.pin-action svg {
  width: 13px;
  height: 13px;
}

.row-trailing.pinned .row-accel,
.row-shell:hover .row-accel,
.row-trailing:focus-within .row-accel {
  opacity: 0;
}

.row-trailing.pinned .pin-action,
.row-shell:hover .pin-action,
.pin-action:focus-visible {
  opacity: 0.82;
  pointer-events: auto;
}

.pin-action:hover,
.pin-action:focus-visible {
  background: color-mix(in srgb, currentColor 14%, transparent);
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
  height: var(--image-row-h);
  border-radius: 3px;
  object-fit: contain;
  object-position: left center;
}

.image-placeholder {
  color: var(--fg-faint);
  font-size: 12px;
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
