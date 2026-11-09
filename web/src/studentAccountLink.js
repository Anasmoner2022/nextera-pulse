// Normalize account identifiers consistently before joining roster data.
function normalize(value) {
  return typeof value === 'string' ? value.trim().replace(/^@/, '').toLocaleLowerCase() : ''
}

/** Match a cohort row to its student profile without ever linking mentors. */
export function findLinkedStudentAccount(student, accounts) {
  const login = normalize(student?.login)
  if (!login) return null

  const students = accounts.filter(account => account.role === 'student')
  const explicitMatches = students.filter(account => normalize(account.studentLogin) === login)
  if (explicitMatches.length === 1) return explicitMatches[0]
  if (explicitMatches.length > 1) return null

  // Older demo accounts may not have a saved studentLogin yet. Use an email
  // local part only when the account has no explicit link and the match is unique.
  const legacyMatches = students.filter(account => {
    if (normalize(account.studentLogin)) return false
    return normalize(account.email?.split('@')[0]) === login
  })
  return legacyMatches.length === 1 ? legacyMatches[0] : null
}

/** Find a student's assigned mentor from either the student link or mentor roster. */
export function findAssignedMentor(student, accounts, mentors) {
  const linkedAccount = findLinkedStudentAccount(student, accounts)
  if (linkedAccount?.mentorId) {
    const linkedMentor = mentors.find(mentor => mentor.id === linkedAccount.mentorId)
    if (linkedMentor) return linkedMentor
  }
  const login = normalize(student?.login)
  if (!login) return null
  return mentors.find(mentor => (mentor.assignedStudentLogins ?? []).some(assignedLogin => normalize(assignedLogin) === login)) ?? null
}
