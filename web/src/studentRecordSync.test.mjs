import assert from 'node:assert/strict'
import { createDemoStudentRecord, syncStudentIdentity } from './studentRecordSync.mjs'

const created = createDemoStudentRecord({ studentId: 90, login: '@ziad.demo', displayName: 'Ziad Demo', enrolledAt: '2026-10-07' })
assert.equal(created.login, 'ziad.demo')
assert.equal(created.displayName, 'Ziad Demo')
assert.equal(created.paceStatus, 'NOT_MEASURABLE')
assert.equal(created.activeAcademicDays, 0)
console.log('4/4 new student record defaults passed')

const existing = { ...created, activeAcademicDays: 12, actualLevel: 4, paceStatus: 'GREEN' }
const renamed = syncStudentIdentity(existing, { login: 'new.login', displayName: 'Updated Name' })
assert.equal(renamed.login, 'new.login')
assert.equal(renamed.displayName, 'Updated Name')
assert.equal(renamed.activeAcademicDays, 12)
assert.equal(renamed.actualLevel, 4)
assert.equal(renamed.paceStatus, 'GREEN')
console.log('5/5 identity sync preserves pace data passed')
