<script lang="ts" setup>
import { computed, onMounted, ref } from 'vue'
import type { statistics } from '../../wailsjs/go/models'
import * as App from '../../wailsjs/go/main/App'

type Period = 'day' | 'week' | 'month' | 'year'
type Metric = 'total' | 'text' | 'image' | 'repeated'

interface MetricDefinition {
  value: Metric
  label: string
}

const periods: Array<{ value: Period; label: string }> = [
  { value: 'day', label: 'Day' },
  { value: 'week', label: 'Week' },
  { value: 'month', label: 'Month' },
  { value: 'year', label: 'Year' },
]

const metrics: MetricDefinition[] = [
  { value: 'total', label: 'All copies' },
  { value: 'text', label: 'Text' },
  { value: 'image', label: 'Images' },
  { value: 'repeated', label: 'Repeated' },
]

// Draw the supporting series first so the stronger total line stays legible
// where Text and All copies follow nearly the same path.
const renderOrder: Metric[] = ['text', 'image', 'repeated', 'total']

const period = ref<Period>('week')
const visibleMetrics = ref<Record<Metric, boolean>>({
  total: true,
  text: true,
  image: true,
  repeated: true,
})
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

function metricValue(counts: statistics.Counts, metric: Metric): number {
  return counts[metric]
}

const visibleMetricList = computed(() => metrics.filter((item) => visibleMetrics.value[item.value]))
const visibleRenderOrder = computed(() => renderOrder.filter((metric) => visibleMetrics.value[metric]))
const maxValue = computed(() => {
  const points = snapshot.value?.points ?? []
  const values = visibleMetricList.value.flatMap((item) =>
    points.map((point) => metricValue(point.counts, item.value)),
  )
  return Math.max(1, ...values)
})
const plotWidth = chart.width - chart.left - chart.right
const plotHeight = chart.height - chart.top - chart.bottom

function pointX(index: number): number {
  const count = Math.max(1, (snapshot.value?.points.length ?? 1) - 1)
  return chart.left + (index / count) * plotWidth
}

function pointY(value: number): number {
  return chart.top + plotHeight - (value / maxValue.value) * plotHeight
}

function polyline(metric: Metric): string {
  return (snapshot.value?.points ?? [])
    .map((point, index) => `${pointX(index)},${pointY(metricValue(point.counts, metric))}`)
    .join(' ')
}

const yTicks = computed(() => {
  const top = maxValue.value
  return [top, Math.round(top / 2), 0].filter((value, index, all) => all.indexOf(value) === index)
})

const xTickIndexes = computed(() => {
  const count = snapshot.value?.points.length ?? 0
  if (count <= 12) return Array.from({ length: count }, (_, index) => index)
  const indexes = [0, Math.round((count - 1) / 4), Math.round((count - 1) / 2), Math.round(((count - 1) * 3) / 4), count - 1]
  return indexes.filter((value, index, all) => all.indexOf(value) === index)
})

const active = computed(() => {
  const index = activePoint.value
  const point = index === null ? undefined : snapshot.value?.points[index]
  if (!point || index === null) return null
  return {
    x: pointX(index),
    label: formatPoint(point.start, true),
    values: visibleMetricList.value.map((item) => ({
      ...item,
      count: metricValue(point.counts, item.value),
      y: pointY(metricValue(point.counts, item.value)),
    })),
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
  const values = visibleMetricList.value
    .map((item) => `${item.label}: ${metricValue(point.counts, item.value).toLocaleString()}`)
    .join(', ')
  return values ? `${formatPoint(point.start, true)}. ${values}` : `${formatPoint(point.start, true)}. No metrics shown.`
}

function toggleMetric(metric: Metric): void {
  visibleMetrics.value[metric] = !visibleMetrics.value[metric]
}

function metricAria(item: MetricDefinition): string {
  const count = snapshot.value?.totals[item.value] ?? 0
  const state = visibleMetrics.value[item.value] ? 'shown' : 'hidden'
  return `${item.label}: ${count.toLocaleString()}, ${state}. Activate to ${state === 'shown' ? 'hide' : 'show'} on chart.`
}

function selectNearestPoint(event: PointerEvent): void {
  const count = snapshot.value?.points.length ?? 0
  if (count === 0) return
  const bounds = (event.currentTarget as SVGSVGElement).getBoundingClientRect()
  const svgX = ((event.clientX - bounds.left) / bounds.width) * chart.width
  const plotX = Math.min(chart.width - chart.right, Math.max(chart.left, svgX))
  activePoint.value = Math.round(((plotX - chart.left) / plotWidth) * Math.max(0, count - 1))
}

function pointHitX(index: number): number {
  const halfWidth = snapshot.value && snapshot.value.points.length > 1
    ? plotWidth / (snapshot.value.points.length - 1) / 2
    : plotWidth / 2
  return Math.max(chart.left, pointX(index) - halfWidth)
}

function pointHitWidth(index: number): number {
  const nextStart = index === (snapshot.value?.points.length ?? 0) - 1
    ? chart.width - chart.right
    : pointHitX(index + 1)
  return nextStart - pointHitX(index)
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
        :class="[`metric-${item.value}`, { hidden: !visibleMetrics[item.value] }]"
        :aria-label="metricAria(item)"
        :aria-pressed="visibleMetrics[item.value]"
        @click="toggleMetric(item.value)"
      >
        <span class="metric-label"><i aria-hidden="true" />{{ item.label }}</span>
        <strong>{{ snapshot.totals[item.value].toLocaleString() }}</strong>
        <span class="metric-state">{{ visibleMetrics[item.value] ? 'Shown' : 'Hidden' }}</span>
      </button>
    </div>

    <p v-if="snapshot" class="chart-hint">Click a metric to hide or show it.</p>

    <div v-if="snapshot" class="chart-wrap" :aria-busy="loading">
      <svg
        class="chart"
        :viewBox="`0 0 ${chart.width} ${chart.height}`"
        role="img"
        :aria-label="`Clipboard activity, ${periodDescription()}. ${visibleMetricList.length} of ${metrics.length} metrics shown.`"
        @pointermove="selectNearestPoint"
        @pointerleave="activePoint = null"
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

        <rect
          class="plot-hit"
          :x="chart.left"
          :y="chart.top"
          :width="plotWidth"
          :height="plotHeight"
        />

        <g v-for="series in visibleRenderOrder" :key="series" :class="`series-${series}`">
          <polyline class="line" :points="polyline(series)" />
          <circle
            v-for="(point, index) in snapshot.points.length <= 12 ? snapshot.points : []"
            :key="String(point.start)"
            class="dot"
            :cx="pointX(index)"
            :cy="pointY(metricValue(point.counts, series))"
            r="2.6"
          />
        </g>

        <g v-if="active && active.values.length" class="active-guide">
          <line :x1="active.x" :x2="active.x" :y1="chart.top" :y2="chart.top + plotHeight" />
          <circle
            v-for="item in active.values"
            :key="item.value"
            :class="`active-dot series-${item.value}`"
            :cx="active.x"
            :cy="item.y"
            r="4.5"
          />
        </g>

        <g v-for="(point, index) in snapshot.points" :key="String(point.start)">
          <rect
            class="keyboard-hit"
            :x="pointHitX(index)"
            :y="chart.top"
            :width="pointHitWidth(index)"
            :height="plotHeight"
            tabindex="0"
            :aria-label="pointAria(point)"
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

        <text
          v-if="visibleMetricList.length === 0"
          class="empty-chart"
          :x="chart.left + plotWidth / 2"
          :y="chart.top + plotHeight / 2"
          text-anchor="middle"
        >
          Select a metric above to show it.
        </text>
      </svg>

      <div
        v-if="active && active.values.length"
        class="tooltip"
        role="status"
        :class="{ 'align-left': active.x < chart.width * 0.3, 'align-right': active.x > chart.width * 0.72 }"
        :style="{ left: `${(active.x / chart.width) * 100}%` }"
      >
        <strong>{{ active.label }}</strong>
        <span v-for="item in active.values" :key="item.value" class="tooltip-row">
          <i :class="`series-${item.value}`" aria-hidden="true" />
          <span>{{ item.label }}</span>
          <b>{{ item.count.toLocaleString() }}</b>
        </span>
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
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 3px;
  padding: 10px 12px;
  border: 1px solid var(--panel-border);
  border-radius: 8px;
  background: var(--field-bg);
  text-align: left;
  transition: opacity 120ms ease, border-color 120ms ease, background 120ms ease;
}

.metric-card:hover {
  background: var(--hover-bg);
}

.metric-card:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.metric-card.hidden {
  opacity: 0.42;
}

.metric-label,
.metric-state {
  color: var(--fg-dim);
  font-size: 11.5px;
}

.metric-label {
  display: flex;
  align-items: center;
  gap: 6px;
}

.metric-label i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

.metric-total .metric-label i { color: var(--series-total); }
.metric-text .metric-label i { color: var(--series-text); }
.metric-image .metric-label i { color: var(--series-image); }
.metric-repeated .metric-label i { color: var(--series-repeated); }

.metric-card.hidden .metric-label i {
  background: transparent;
  border: 1.5px solid currentColor;
}

.metric-card strong {
  color: var(--fg);
  font-size: 21px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.metric-state {
  position: absolute;
  top: 10px;
  right: 11px;
  font-size: 10px;
  opacity: 0;
}

.metric-card.hidden .metric-state {
  opacity: 1;
}

.chart-hint {
  margin: 7px 0 -7px;
  color: var(--fg-dim);
  font-size: 10.5px;
  text-align: center;
}

.chart-wrap {
  position: relative;
  margin-top: 14px;
  padding: 5px;
  border: 1px solid var(--panel-border);
  border-radius: 9px;
  background: color-mix(in srgb, var(--field-bg) 60%, transparent);
}

.statistics {
  --series-total: #4d91ff;
  --series-text: #2dc7bd;
  --series-image: #f3a630;
  --series-repeated: #bf7cff;
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
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 1.8;
}

.dot {
  fill: currentColor;
  pointer-events: none;
}

.series-total { color: var(--series-total); }
.series-text { color: var(--series-text); }
.series-image { color: var(--series-image); }
.series-repeated { color: var(--series-repeated); }

.series-total .line {
  stroke-width: 2.8;
}

.series-repeated .line {
  stroke-dasharray: 6 4;
}

.plot-hit,
.keyboard-hit {
  fill: transparent;
}

.keyboard-hit {
  outline: none;
}

.keyboard-hit:focus {
  fill: color-mix(in srgb, var(--accent) 7%, transparent);
  stroke: var(--accent);
  stroke-width: 1;
}

.active-guide line {
  stroke: color-mix(in srgb, var(--fg) 48%, transparent);
  stroke-dasharray: 3 3;
  stroke-width: 1;
  pointer-events: none;
}

.active-dot {
  fill: var(--card-bg);
  stroke: currentColor;
  stroke-width: 2.2;
  pointer-events: none;
}

.empty-chart {
  fill: var(--fg-dim);
  font-size: 12px;
}

.tooltip {
  position: absolute;
  z-index: 2;
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 154px;
  padding: 6px 8px;
  border: 1px solid var(--panel-border);
  border-radius: 6px;
  background: var(--card-bg);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.25);
  top: 16px;
  transform: translateX(-50%);
  pointer-events: none;
}

.tooltip strong {
  margin-bottom: 2px;
  font-size: 11.5px;
}

.tooltip.align-left {
  transform: translateX(8px);
}

.tooltip.align-right {
  transform: translateX(calc(-100% - 8px));
}

.tooltip-row {
  display: grid;
  grid-template-columns: 8px 1fr auto;
  align-items: center;
  gap: 6px;
  color: var(--fg-dim);
  font-size: 10.5px;
  white-space: nowrap;
}

.tooltip-row i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
}

.tooltip-row b {
  color: var(--fg);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

@media (prefers-color-scheme: light) {
  .statistics {
    --series-total: #2563d8;
    --series-text: #087f78;
    --series-image: #a95d00;
    --series-repeated: #8436bd;
  }
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
