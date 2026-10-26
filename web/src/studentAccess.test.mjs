import assert from 'node:assert/strict'
import { assignStudentToMentor, visibleStudentsForRole } from './studentAccess.mjs'

const rows = [{ login: 'alpha' }, { login: 'Beta' }, { login: 'gamma' }]
assert.deepEqual(visibleStudentsForRole('mentor', rows, ['@ALPHA']), [rows[0]])
assert.deepEqual(visibleStudentsForRole('mentor', rows, ['@ALPHA', 'gamma']), [rows[0], rows[2]])
assert.deepEqual(visibleStudentsForRole('manager', rows, []), rows)
assert.deepEqual(visibleStudentsForRole('student', rows, [], '@BETA'), [rows[1]])
console.log('4/4 role visibility checks passed')

const mentors = [
  { id: 'm1', assignedStudentLogins: ['alpha'] },
  { id: 'm2', assignedStudentLogins: ['beta'] },
]
const moved = assignStudentToMentor(mentors, 'm2', '@ALPHA', true)
assert.deepEqual(moved[0].assignedStudentLogins, [])
assert.deepEqual(moved[1].assignedStudentLogins, ['beta', 'alpha'])
const unassigned = assignStudentToMentor(moved, 'm2', 'alpha', false)
assert.deepEqual(unassigned[1].assignedStudentLogins, ['beta'])
console.log('2/2 single-mentor assignment checks passed')
