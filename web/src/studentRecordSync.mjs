/** Create a synthetic student row without inventing learning measurements. */
export function createDemoStudentRecord({ studentId, login, displayName, enrolledAt = new Date().toISOString().slice(0, 10) }) {
  const cleanLogin = login.trim().replace(/^@/, '')
  return {
    studentId,
    login: cleanLogin,
    displayName: displayName.trim(),
    email: `${cleanLogin.replace(/[^a-z0-9._-]/gi, '')}@example.com`,
    age: 17 + (studentId % 6),
    phone: `+20 10 ${String(studentId).slice(-4)} ${String(studentId * 31).slice(-4)}`,
    enrolledAt,
    activeAcademicDays: 0,
    curriculumMonth: 0,
    actualLevel: 0,
    minimumLevel: null,
    expectedLevel: null,
    checkpointTarget: null,
    gapToMinimum: null,
    gapToExpected: null,
    paceStatus: 'NOT_MEASURABLE',
    sourceStatus: 'active',
    healthReasonCodes: [],
  }
}

/** Update identity only; progress values continue to belong to the data source. */
export function syncStudentIdentity(row, { login, displayName }) {
  return {
    ...row,
    login: login.trim().replace(/^@/, ''),
    displayName: displayName.trim(),
  }
}
