<script lang="ts" setup>
/** The hover detail card: full preview plus provenance, mirroring the
 *  reference app's tooltip. Rendered inside the window (the popup cannot draw
 *  outside its own bounds) and vertically aligned with the hovered row. */
import { computed } from 'vue'
import type { store } from '../../wailsjs/go/models'
import { formatBytes, formatCopyTime, itemLabel, textStats } from '../lib/format'
import { combo, sym } from '../lib/keys'

const props = defineProps<{
  item: store.Item
  /** Offset from the top of the panel, in px. The parent measures this card and
   *  clamps the value so the card always fits inside the list area. */
  top: number
}>()

const label = computed(() => itemLabel(props.item))

const dimensions = computed(() =>
  props.item.kind === 'image' && props.item.imageW && props.item.imageH
    ? `${props.item.imageW} × ${props.item.imageH}`
    : '',
)

const stats = computed(() =>
  props.item.kind === 'image'
    ? [dimensions.value, formatBytes(props.item.bytes)].filter(Boolean).join(' · ')
    : textStats(props.item.text),
)

const pinHint = computed(() =>
  `Press ${combo(sym.alt, 'P')} to ${props.item.pinned ? 'unpin' : 'pin'}.`,
)
const deleteHint = computed(() => `Press ${combo(sym.alt, sym.del)} to delete.`)
</script>

<template>
  <div class="bubble" :style="{ top: `${top}px` }">
    <div class="preview">
      <img v-if="item.kind === 'image' && item.thumb" :src="item.thumb" alt="" />
      <p v-else class="text">{{ label }}</p>
    </div>

    <div class="hairline" />

    <dl class="meta">
      <template v-if="item.sourceApp">
        <dt>Application:</dt>
        <dd class="app">
          <img v-if="item.sourceIcon" :src="item.sourceIcon" alt="" class="app-icon" />
          <span>{{ item.sourceApp }}</span>
        </dd>
      </template>

      <dt>First copy time:</dt>
      <dd>{{ formatCopyTime(item.firstCopy) }}</dd>

      <dt>Last copy time:</dt>
      <dd>{{ formatCopyTime(item.lastCopy) }}</dd>

      <dt>Number of copies:</dt>
      <dd>{{ item.copyCount }}</dd>

      <template v-if="stats">
        <dt>{{ item.kind === 'image' ? 'Image:' : 'Length:' }}</dt>
        <dd>{{ stats }}</dd>
      </template>
    </dl>

    <p class="hints">
      {{ pinHint }}<br />
      {{ deleteHint }}
    </p>
  </div>
</template>

<style scoped>
.bubble {
  position: absolute;
  left: 8px;
  width: 58%;
  max-width: 400px;
  z-index: 20;
  padding: 12px 14px;
  border-radius: 10px;
  background: var(--bubble-bg);
  border: 1px solid var(--bubble-border);
  box-shadow: 0 10px 32px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  pointer-events: none;
}

.preview {
  max-height: 110px;
  overflow: hidden;
}

.preview img {
  display: block;
  max-width: 100%;
  max-height: 110px;
  border-radius: 4px;
  object-fit: contain;
}

.text {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
  /* Long payloads get clamped rather than growing the card off-screen. */
  display: -webkit-box;
  -webkit-line-clamp: 5;
  line-clamp: 5;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

.hairline {
  margin: 10px 0;
}

.meta {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 2px 8px;
  margin: 0;
  font-size: 12px;
}

.meta dt {
  color: var(--fg-dim);
  white-space: nowrap;
}

.meta dd {
  margin: 0;
  color: var(--fg);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app {
  display: flex;
  align-items: center;
  gap: 5px;
}

.app-icon {
  width: 14px;
  height: 14px;
  flex: none;
  border-radius: 2px;
}

.hints {
  margin: 10px 0 0;
  font-size: 11.5px;
  color: var(--fg-dim);
  line-height: 1.5;
}
</style>
