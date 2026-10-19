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
