<script lang="ts" setup>
/** The detail card: full preview plus provenance for the entry under the
 *  cursor (or, with the mouse away, the selected one). The popup floats it to
 *  the left of the list, level with the row, so the list itself stays narrow
 *  and the row being inspected stays visible. */
import { computed } from 'vue'
import type { store } from '../../wailsjs/go/models'
import { formatBytes, formatCopyTime, itemLabel, textStats } from '../lib/format'
import { combo, sym } from '../lib/keys'

const props = defineProps<{
  /** The entry to describe, or null when the list is empty. */
  item: store.Item | null
}>()

const label = computed(() => (props.item ? itemLabel(props.item) : ''))

const dimensions = computed(() =>
  props.item?.kind === 'image' && props.item.imageW && props.item.imageH
    ? `${props.item.imageW} × ${props.item.imageH}`
    : '',
)

const stats = computed(() => {
  if (!props.item) return ''
  return props.item.kind === 'image'
    ? [dimensions.value, formatBytes(props.item.bytes)].filter(Boolean).join(' · ')
    : textStats(props.item.text)
})

const pinHint = computed(() =>
  `Press ${combo(sym.alt, 'P')} to ${props.item?.pinned ? 'unpin' : 'pin'}.`,
)
const deleteHint = computed(() => `Press ${combo(sym.alt, sym.del)} to delete.`)
</script>

<template>
  <aside class="detail">
    <p v-if="!item" class="placeholder">Nothing to preview</p>

    <template v-else>
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
    </template>
  </aside>
</template>

<style scoped>
/* The card that carries this pane owns the width, background and border. */
.detail {
  padding: 10px 12px;
}

.placeholder {
  margin: 0;
  padding-top: 14px;
  color: var(--fg-faint);
  font-size: 12.5px;
}

.preview {
  max-height: 170px;
  overflow: hidden;
}

.preview img {
  display: block;
  max-width: 100%;
  max-height: 170px;
  border-radius: 4px;
  object-fit: contain;
}

.text {
  margin: 0;
  font-size: 13px;
  line-height: 1.45;
  /* Long payloads get clamped rather than pushing the metadata out of view. */
  display: -webkit-box;
  -webkit-line-clamp: 8;
  line-clamp: 8;
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
