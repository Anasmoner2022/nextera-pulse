import assert from 'node:assert/strict'
import { findLinkedStudentAccount } from './studentAccountLink.js'

const student = { studentId: 1, login: 'Ziad.Dev', displayName: 'Ziad' }
const linked = { id: 'student-1', role: 'student', studentLogin: 'ziad.dev', avatar: 'data:image/jpeg;base64,photo' }

assert.equal(findLinkedStudentAccount(student, [linked]), linked, 'profile account is linked by academy login, case-insensitively')
assert.equal(findLinkedStudentAccount(student, [{ ...linked, studentLogin: 'other.login' }, { id: 'mentor', role: 'mentor', studentLogin: 'ziad.dev' }]), null, 'a different explicit student link and mentor account are never matched')
assert.equal(findLinkedStudentAccount(student, [{ id: 'legacy', role: 'student', email: 'ziad.dev@example.com' }])?.id, 'legacy', 'older accounts can link by unique email local part')
assert.equal(findLinkedStudentAccount(student, [{ id: 'legacy-a', role: 'student', email: 'ziad.dev@a.test' }, { id: 'legacy-b', role: 'student', email: 'ziad.dev@b.test' }]), null, 'ambiguous legacy matches are ignored')

console.log('4/4 student account linkage checks passed')
