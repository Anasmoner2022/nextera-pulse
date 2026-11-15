// North Star KPI (review C4): on-track rate = GREEN / measurable students.
//
// Live mode never computes this in the browser: the API returns it from
// GET /api/v1/kpi/on-track (internal/pace/kpi.go). This copy of the same
// formula is only for demo mode, on mock data and a mock status policy.
//
// A student is measurable when their pace status compares them with a monthly
// target (GREEN, YELLOW or RED) and their source status is not flagged
// excludeFromActiveKpis in the status policy. With no measurable students the
// rate is null (shown as "—"), never 0%.

export const measurablePaceStatuses = ['GREEN', 'YELLOW', 'RED']

export function computeOnTrack(students, statusPolicy) {
  const excluded = new Set(statusPolicy.filter(policy => policy.excludeFromActiveKpis).map(policy => policy.recordType))
  const kpi = { onTrack: 0, measurable: 0, rate: null, hasMeasurable: false, total: students.length, excludedByPolicy: 0, statusCounts: {}, measurableCounts: { GREEN: 0, YELLOW: 0, RED: 0 } }
  for (const student of students) {
    const status = student.paceStatus || 'UNKNOWN'
    kpi.statusCounts[status] = (kpi.statusCounts[status] ?? 0) + 1
    if (excluded.has(student.sourceStatus)) { kpi.excludedByPolicy++; continue }
    if (!measurablePaceStatuses.includes(status)) continue
    kpi.measurable++
    kpi.measurableCounts[status]++
    if (status === 'GREEN') kpi.onTrack++
  }
  if (kpi.measurable > 0) {
    kpi.rate = kpi.onTrack / kpi.measurable * 100
    kpi.hasMeasurable = true
  }
  return kpi
}

// Text for one KPI card: the rate and "X of N measurable on pace", or a clear
// loading, error or no-measurable-students state instead of 0%.
export function describeOnTrack(state, kpi) {
  if (state.status === 'loading') return { value: '…', foot: 'Loading the on-track rate…', tone: 'loading' }
  if (state.status === 'error') return { value: '—', foot: `On-track rate unavailable${state.error ? `: ${state.error}` : ''}`, tone: 'error' }
  if (!kpi) return { value: '—', foot: 'No on-track rate for this scope', tone: 'empty' }
  if (!kpi.hasMeasurable || kpi.rate === null) return { value: '—', foot: `No measurable students (0 of ${kpi.total} on a measurable pace)`, tone: 'empty' }
  return { value: `${Math.round(kpi.rate)}%`, foot: `${kpi.onTrack} of ${kpi.measurable} measurable on pace`, tone: 'ready' }
}
