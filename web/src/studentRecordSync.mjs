export function createDemoStudentRecord({ studentId, login, displayName, enrolledAt = new Date().toISOString().slice(0, 10) }) {
  return {
    studentId,
    login: login.trim().replace(/^@/, ''),
    displayName: displayName.trim(),
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

export function syncStudentIdentity(row, { login, displayName }) {
  return {
    ...row,
    login: login.trim().replace(/^@/, ''),
    displayName: displayName.trim(),
  }
}
