/** @typedef {{ id: string, passwordHash?: string, [key: string]: unknown }} StoredAccount */

/** @param {StoredAccount} account */
function removeRetiredProfileFields(account) {
  const { bio: _bio, posts: _posts, ...clean } = account
  if (Array.isArray(clean.permissions)) {
    clean.permissions = clean.permissions.filter(permission => permission !== 'community')
  }
  return clean
}

/** @param {Storage} storage @param {StoredAccount} account @param {string} key */
export function persistProfileAccount(storage, account, key = 'pulse-accounts') {
  const saved = readStoredAccounts(storage, key)
  const previous = saved.find(item => item.id === account.id)
  const updated = removeRetiredProfileFields({ ...previous, ...account })
  if (previous?.passwordHash) updated.passwordHash = previous.passwordHash
  const next = [...saved.filter(item => item.id !== account.id), updated]
  storage.setItem(key, JSON.stringify(next))
  return next
}

/** @param {Storage} storage @param {string} [key] @returns {StoredAccount[]} */
export function readStoredAccounts(storage, key = 'pulse-accounts') {
  try {
    const value = JSON.parse(storage.getItem(key) || '[]')
    return Array.isArray(value)
      ? value.filter(item => item && typeof item.id === 'string').map(removeRetiredProfileFields)
      : []
  } catch {
    return []
  }
}

/** @param {string} password */
export async function hashAccountPassword(password) {
  const bytes = new TextEncoder().encode(password)
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('')
}

/** @param {Storage} storage @param {string} email @param {string} password */
export async function authenticateStoredAccount(storage, email, password) {
  const account = readStoredAccounts(storage).find(item => typeof item.email === 'string' && item.email.toLowerCase() === email.trim().toLowerCase())
  if (!account?.passwordHash || await hashAccountPassword(password) !== account.passwordHash) return null
  const { passwordHash: _passwordHash, ...session } = account
  return session
}
