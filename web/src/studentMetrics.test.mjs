import assert from 'node:assert/strict'
import { computeOnTrack, describeOnTrack } from './studentMetrics.mjs'

const policy = [
  { recordType: 'restricted', excludeFromActiveKpis: true },
  { recordType: 'left-school', excludeFromActiveKpis: true },
  { recordType: 'away', excludeFromActiveKpis: false },
]
let passed = 0
const check = (fn) => { fn(); passed++ }

// Mixed statuses: GREEN / (GREEN + YELLOW + RED), YELLOW is not on track.
check(() => {
  const kpi = computeOnTrack([
    { paceStatus: 'GREEN', sourceStatus: 'active' }, { paceStatus: 'GREEN', sourceStatus: 'away' },
    { paceStatus: 'YELLOW', sourceStatus: 'active' }, { paceStatus: 'RED', sourceStatus: 'active' },
    { paceStatus: 'PAUSED', sourceStatus: 'away' }, { paceStatus: 'INACTIVE', sourceStatus: 'active' },
    { paceStatus: 'NOT_MEASURABLE', sourceStatus: 'active' }, { paceStatus: 'ONBOARDING', sourceStatus: 'active' },
    { paceStatus: 'GREEN', sourceStatus: 'restricted' },
  ], policy)
  assert.equal(kpi.onTrack, 2)
  assert.equal(kpi.measurable, 4)
  assert.equal(kpi.rate, 50)
  assert.equal(kpi.excludedByPolicy, 1)
  assert.equal(kpi.measurableCounts.YELLOW, 1)
  assert.equal(kpi.statusCounts.GREEN, 3)
})

// Zero denominator: null rate, never 0%.
check(() => {
  const kpi = computeOnTrack([{ paceStatus: 'PAUSED', sourceStatus: 'away' }, { paceStatus: 'GREEN', sourceStatus: 'left-school' }], policy)
  assert.equal(kpi.rate, null)
  assert.equal(kpi.hasMeasurable, false)
  assert.equal(describeOnTrack({ status: 'ready' }, kpi).value, '—')
  assert.equal(computeOnTrack([], policy).rate, null)
})

// Exclusion comes from the policy rows, not a list in code.
check(() => {
  const rows = [{ paceStatus: 'GREEN', sourceStatus: 'observation' }, { paceStatus: 'RED', sourceStatus: 'active' }]
  assert.equal(computeOnTrack(rows, []).rate, 50)
  assert.equal(computeOnTrack(rows, [{ recordType: 'observation', excludeFromActiveKpis: true }]).rate, 0)
  assert.equal(computeOnTrack(rows, [{ recordType: 'observation', excludeFromActiveKpis: false }]).measurable, 2)
})

// Card text: X of N measurable, and loading / error instead of 0.
check(() => {
  const kpi = computeOnTrack([{ paceStatus: 'GREEN', sourceStatus: 'active' }, { paceStatus: 'YELLOW', sourceStatus: 'active' }, { paceStatus: 'RED', sourceStatus: 'active' }], policy)
  assert.deepEqual(describeOnTrack({ status: 'ready' }, kpi), { value: '33%', foot: '1 of 3 measurable on pace', tone: 'ready' })
  assert.equal(describeOnTrack({ status: 'loading' }, null).value, '…')
  assert.equal(describeOnTrack({ status: 'error', error: 'API returned 500' }, null).tone, 'error')
})

console.log(`${passed}/4 on-track KPI checks passed`)
