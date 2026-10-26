const normalizeLogin = (value = '') => value.trim().replace(/^@/, '').toLowerCase()

export function visibleStudentsForRole(role, rows, assignedLogins = [], studentLogin = '') {
  if (role === 'manager') return rows
  if (role === 'mentor') {
    const assigned = new Set(assignedLogins.map(normalizeLogin))
    return rows.filter(row => assigned.has(normalizeLogin(row.login)))
  }
  if (role === 'student') {
    const login = normalizeLogin(studentLogin)
    return rows.filter(row => normalizeLogin(row.login) === login)
  }
  return []
}

export function assignStudentToMentor(mentors, mentorId, login, assigned) {
  const normalized = normalizeLogin(login)
  return mentors.map(mentor => {
    const logins = new Set((mentor.assignedStudentLogins ?? []).map(normalizeLogin))
    if (assigned) {
      if (mentor.id === mentorId) logins.add(normalized)
      else logins.delete(normalized)
    } else if (mentor.id === mentorId) {
      logins.delete(normalized)
    }
    return { ...mentor, assignedStudentLogins: [...logins] }
  })
}
