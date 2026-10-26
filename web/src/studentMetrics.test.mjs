import assert from 'node:assert/strict'
import { isInActiveKpiPopulation } from './studentMetrics.mjs'

for (const sourceStatus of ['restricted', 'blocked-long', 'ban', 'left-school']) {
  assert.equal(isInActiveKpiPopulation({ sourceStatus }), false, `${sourceStatus} must be excluded`)
}

for (const sourceStatus of ['active', 'away', 'temporary-notice', 'observation']) {
  assert.equal(isInActiveKpiPopulation({ sourceStatus }), true, `${sourceStatus} must remain included`)
}

console.log('8/8 active KPI source-status rules passed')
