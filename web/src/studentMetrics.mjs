const excludedFromActiveKpis = new Set(['restricted', 'blocked-long', 'ban', 'left-school'])

export function isInActiveKpiPopulation(student) {
  return !excludedFromActiveKpis.has(student.sourceStatus)
}
