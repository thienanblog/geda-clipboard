<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import type { statistics } from '../../wailsjs/go/models'
import * as App from '../../wailsjs/go/main/App'

type Period = 'day' | 'week' | 'month' | 'year'
type Metric = 'total' | 'text' | 'image' | 'repeated'

const periods: Array<{ value: Period; label: string }> = [
  { value: 'day', label: 'Day' },
  { value: 'week', label: 'Week' },
  { value: 'month', label: 'Month' },
  { value: 'year', label: 'Year' },
]

const metrics: Array<{ value: Metric; label: string }> = [
  { value: 'total', label: 'All copies' },
  { value: 'text', label: 'Text' },
  { value: 'image', label: 'Images' },
  { value: 'repeated', label: 'Repeated' },
]

const period = ref<Period>('week')
const metric = ref<Metric>('total')
const snapshot = ref<statistics.Snapshot | null>(null)
const loading = ref(false)
const error = ref('')
const activePoint = ref<number | null>(null)
let loadSequence = 0

const chart = {
  width: 640,
  height: 230,
  left: 42,
  right: 14,
  top: 16,
  bottom: 36,
}

function metricValue(counts: statistics.Counts, selected = metric.value): number {
  return counts[selected]
}

const values = computed(() => (snapshot.value?.points ?? []).map((point) => metricValue(point.counts)))
const maxValue = computed(() => Math.max(1, ...values.value))
const plotWidth = chart.width - chart.left - chart.right
const plotHeight = chart.height - chart.top - chart.bottom

function pointX(index: number): number {
  const count = Math.max(1, values.value.length - 1)
  return chart.left + (index / count) * plotWidth
}

function pointY(value: number): number {
  return chart.top + plotHeight - (value / maxValue.value) * plotHeight
}

const polyline = computed(() =>
  values.value.map((value, index) => `${pointX(index)},${pointY(value)}`).join(' '),
)

const yTicks = computed(() => {
  const top = maxValue.value
  return [top, Math.round(top / 2), 0].filter((value, index, all) => all.indexOf(value) === index)
})

const xTickIndexes = computed(() => {
  const count = values.value.length
  if (count <= 12) return Array.from({ length: count }, (_, index) => index)
  const indexes = [0, Math.round((count - 1) / 4), Math.round((count - 1) / 2), Math.round(((count - 1) * 3) / 4), count - 1]
  return indexes.filter((value, index, all) => all.indexOf(value) === index)
})

const active = computed(() => {
  const index = activePoint.value
  const point = index === null ? undefined : snapshot.value?.points[index]
  if (!point || index === null) return null
  return {
    index,
    x: pointX(index),
    y: pointY(metricValue(point.counts)),
    value: metricValue(point.counts),
    label: formatPoint(point.start, true),
  }
})

function formatPoint(raw: unknown, detailed = false): string {
  const date = new Date(String(raw))
  if (period.value === 'day') {
    return new Intl.DateTimeFormat(undefined, { hour: 'numeric', minute: detailed ? '2-digit' : undefined }).format(date)
  }
  if (period.value === 'year') {
    return new Intl.DateTimeFormat(undefined, { month: detailed ? 'long' : 'short', year: detailed ? 'numeric' : undefined }).format(date)
  }
  return new Intl.DateTimeFormat(undefined, {
    weekday: period.value === 'week' && !detailed ? 'short' : undefined,
    month: detailed ? 'short' : undefined,
    day: 'numeric',
  }).format(date)
}

function pointAria(point: statistics.Point): string {
  return `${formatPoint(point.start, true)}: ${metricValue(point.counts)} ${metrics.find((item) => item.value === metric.value)?.label.toLowerCase() ?? 'copies'}`
}

function periodDescription(): string {
  if (period.value === 'day') return 'Today by hour'
  if (period.value === 'week') return 'Last 7 days'
  if (period.value === 'month') return 'Last 30 days'
  return 'Last 12 months'
}

async function load(): Promise<void> {
  const sequence = ++loadSequence
  const requestedPeriod = period.value
  loading.value = true
  error.value = ''
  activePoint.value = null
  try {
    const result = await App.GetStatistics(requestedPeriod)
    if (sequence === loadSequence) snapshot.value = result
  } catch (err) {
    if (sequence === loadSequence) error.value = String(err)
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

async function selectPeriod(value: Period): Promise<void> {
  period.value = value
  await load()
}

async function reset(): Promise<void> {
  if (!window.confirm('Reset all local statistics? Clipboard history will be kept.')) return
  try {
    await App.ResetStatistics()
    await load()
  } catch (err) {
    error.value = String(err)
  }
}

function startedLabel(): string {
  if (!snapshot.value?.startedAt) return 'Statistics begin with your next copy.'
  const date = new Date(String(snapshot.value.startedAt))
  return `Recording locally since ${new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(date)}.`
}

onMounted(load)
</script>

<template>
  <div class="statistics">
    <div class="statistics-heading">
      <div>
        <h1>Clipboard statistics</h1>
        <p>{{ periodDescription() }} · {{ startedLabel() }}</p>
      </div>
      <div class="periods" aria-label="Statistics period">
        <button
          v-for="item in periods"
          :key="item.value"
          type="button"
          :class="{ active: period === item.value }"
          @click="selectPeriod(item.value)"
        >
          {{ item.label }}
        </button>
      </div>
    </div>

    <p v-if="error" class="stats-error">{{ error }}</p>

    <div v-if="snapshot" class="metric-grid">
      <button
        v-for="item in metrics"
        :key="item.value"
        type="button"
        class="metric-card"
        :class="{ active: metric === item.value }"
        :aria-pressed="metric === item.value"
        @click="metric = item.value"
      >
        <span>{{ item.label }}</span>
        <strong>{{ snapshot.totals[item.value].toLocaleString() }}</strong>
      </button>
    </div>

    <div v-if="snapshot" class="chart-wrap" :aria-busy="loading">
      <svg
        class="chart"
        :viewBox="`0 0 ${chart.width} ${chart.height}`"
        role="img"
        :aria-label="`${metrics.find((item) => item.value === metric)?.label}, ${periodDescription()}`"
      >
        <g class="grid-lines">
          <template v-for="tick in yTicks" :key="tick">
            <line
              :x1="chart.left"
              :x2="chart.width - chart.right"
              :y1="pointY(tick)"
              :y2="pointY(tick)"
            />
            <text :x="chart.left - 9" :y="pointY(tick) + 4" text-anchor="end">{{ tick }}</text>
          </template>
        </g>

        <polyline class="line" :points="polyline" />

        <g v-for="(point, index) in snapshot.points" :key="String(point.start)">
          <circle class="dot" :cx="pointX(index)" :cy="pointY(metricValue(point.counts))" r="3" />
          <circle
            class="hit"
            :cx="pointX(index)"
            :cy="pointY(metricValue(point.counts))"
            r="10"
            tabindex="0"
            :aria-label="pointAria(point)"
            @mouseenter="activePoint = index"
            @mouseleave="activePoint = null"
            @focus="activePoint = index"
            @blur="activePoint = null"
          />
        </g>

        <g class="x-labels">
          <text
            v-for="index in xTickIndexes"
            :key="index"
            :x="pointX(index)"
            :y="chart.height - 10"
            :text-anchor="index === 0 ? 'start' : index === snapshot.points.length - 1 ? 'end' : 'middle'"
          >
            {{ formatPoint(snapshot.points[index].start) }}
          </text>
        </g>
      </svg>

      <div
        v-if="active"
        class="tooltip"
        role="status"
        :style="{ left: `${(active.x / chart.width) * 100}%`, top: `${(active.y / chart.height) * 100}%` }"
      >
        <strong>{{ active.value.toLocaleString() }}</strong>
        <span>{{ active.label }}</span>
      </div>
    </div>

    <p class="storage-note">
      Only content-free counters are stored on this device. The file keeps at most
      {{ snapshot?.retentionDays ?? 370 }} days and does not grow with the number of copies.
    </p>

    <button class="reset" type="button" @click="reset">Reset statistics…</button>
  </div>
</template>

<style scoped>
.statistics {
  padding: 16px 2px 24px;
}

.statistics-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

h1 {
  margin: 0;
  font-size: 19px;
  font-weight: 600;
}

.statistics-heading p,
.storage-note {
  margin: 4px 0 0;
  color: var(--fg-dim);
  font-size: 11.5px;
}

.periods {
  display: flex;
  padding: 2px;
  border-radius: 7px;
  background: var(--field-bg);
}

.periods button {
  padding: 4px 10px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  font-size: 12px;
}

.periods button.active {
  background: var(--accent);
  color: var(--accent-fg);
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 9px;
  margin-top: 18px;
}

.metric-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
  padding: 10px 12px;
  border: 1px solid var(--panel-border);
  border-radius: 8px;
  background: var(--field-bg);
  text-align: left;
}

.metric-card.active {
  border-color: var(--accent);
  background: color-mix(in srgb, var(--accent) 13%, var(--field-bg));
}

.metric-card span {
  color: var(--fg-dim);
  font-size: 11.5px;
}

.metric-card strong {
  font-size: 21px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.chart-wrap {
  position: relative;
  margin-top: 14px;
  padding: 5px;
  border: 1px solid var(--panel-border);
  border-radius: 9px;
  background: color-mix(in srgb, var(--field-bg) 60%, transparent);
}

.chart {
  display: block;
  width: 100%;
  height: 250px;
  overflow: visible;
}

.grid-lines line {
  stroke: var(--hairline);
  stroke-width: 1;
}

.grid-lines text,
.x-labels text {
  fill: var(--fg-dim);
  font-size: 10.5px;
}

.line {
  fill: none;
  stroke: var(--accent);
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 2.5;
}

.dot {
  fill: var(--accent);
  pointer-events: none;
}

.hit {
  fill: transparent;
  outline: none;
}

.hit:focus {
  fill: color-mix(in srgb, var(--accent) 22%, transparent);
  stroke: var(--accent);
  stroke-width: 1;
}

.tooltip {
  position: absolute;
  z-index: 2;
  display: flex;
  flex-direction: column;
  min-width: 92px;
  padding: 6px 8px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--card-bg);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
  transform: translate(-50%, calc(-100% - 9px));
  pointer-events: none;
}

.tooltip strong {
  font-size: 14px;
}

.tooltip span {
  color: var(--fg-dim);
  font-size: 10.5px;
  white-space: nowrap;
}

.storage-note {
  margin-top: 11px;
}

.reset {
  margin-top: 12px;
  padding: 5px 12px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--field-bg);
  color: var(--fg-dim);
}

.stats-error {
  color: var(--danger);
}
</style>
