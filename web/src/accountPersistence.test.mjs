import assert from 'node:assert/strict'
import { authenticateStoredAccount, hashAccountPassword, persistProfileAccount, readStoredAccounts } from './accountPersistence.js'
import { findLinkedStudentAccount } from './studentAccountLink.js'

const cases=[]
function test(name,run){cases.push({name,run})}

function memoryStorage(seed = {}) {
  const values = new Map(Object.entries(seed))
  return {
    getItem: key => values.has(key) ? values.get(key) : null,
    setItem: (key, value) => values.set(key, String(value)),
    removeItem: key => values.delete(key),
  }
}

test('profile photo persists across a reload without removing login credentials', async () => {
  const storage = memoryStorage()
  const account = { id: 'zeyad-1', name: 'Zeyad', email: 'zeyad@example.com', role: 'student', studentLogin: 'zeyad.login', passwordHash: await hashAccountPassword('temporary-password'), permissions: ['viewProgress'] }
  persistProfileAccount(storage, account)
  const sessionProfile = { ...account, avatar: 'data:image/jpeg;base64,compressed', quote: 'Learning every day.' }
  delete sessionProfile.passwordHash
  persistProfileAccount(storage, sessionProfile)

  const afterReload = readStoredAccounts(storage)
  assert.equal(afterReload.length, 1)
  assert.equal(afterReload[0].avatar, 'data:image/jpeg;base64,compressed')
  assert.equal(afterReload[0].quote, 'Learning every day.')
  assert.equal(afterReload[0].passwordHash, account.passwordHash)
  const linkedAccount = findLinkedStudentAccount({ login: 'ZEYAD.LOGIN' }, afterReload)
  assert.equal(linkedAccount?.avatar, 'data:image/jpeg;base64,compressed')
  const session=await authenticateStoredAccount(storage,'ZEYAD@example.com','temporary-password')
  assert.equal(session?.id,'zeyad-1')
  assert.equal(session?.role,'student')
  assert.deepEqual(session?.permissions,['viewProgress'])
  assert.equal('passwordHash' in session,false)
  assert.equal(await authenticateStoredAccount(storage,'zeyad@example.com','wrong-password'),null)
})

test('legacy community posts and bio are removed from stored profile data', () => {
  const storage = memoryStorage({ 'pulse-accounts': JSON.stringify([{ id: 'legacy', name: 'Legacy', permissions: ['viewProgress', 'community'], bio: 'Old bio', posts: [{ text: 'Old post' }], quote: 'Keep learning.' }]) })
  const [account] = readStoredAccounts(storage)
  assert.equal(account.bio, undefined)
  assert.equal(account.posts, undefined)
  assert.deepEqual(account.permissions, ['viewProgress'])
  assert.equal(account.quote, 'Keep learning.')
})

test('updating one profile leaves other accounts unchanged', () => {
  const storage = memoryStorage()
  const first = { id: 'one', name: 'One', email: 'one@example.com', role: 'student', passwordHash: 'hash-one' }
  const second = { id: 'two', name: 'Two', email: 'two@example.com', role: 'mentor', passwordHash: 'hash-two', avatar: 'existing-photo' }
  persistProfileAccount(storage, first)
  persistProfileAccount(storage, second)
  persistProfileAccount(storage, { id: 'one', name: 'One updated', avatar: 'new-photo' })

  const saved = readStoredAccounts(storage)
  assert.equal(saved.length, 2)
  assert.deepEqual(saved.find(item => item.id === 'one'), { ...first, name: 'One updated', avatar: 'new-photo' })
  assert.deepEqual(saved.find(item => item.id === 'two'), second)
})

test('profile-only demo accounts are kept between mentor preview sessions', () => {
  const storage = memoryStorage()
  persistProfileAccount(storage, { id: 'demo-mentor', name: 'Mentor demo', email: 'mentor@nextera.io', role: 'mentor', permissions: ['manageAccounts'], avatar: 'mentor-photo' })
  assert.equal(readStoredAccounts(storage).find(item => item.id === 'demo-mentor')?.avatar, 'mentor-photo')
})

test('invalid local account JSON is treated as an empty account list', () => {
  const storage = memoryStorage({ 'pulse-accounts': '{not-json' })
  assert.deepEqual(readStoredAccounts(storage), [])
})

let failures=0
for(const {name,run} of cases){try{await run();process.stdout.write(`PASS ${name}\n`)}catch(error){failures++;process.stderr.write(`FAIL ${name}\n${error.stack||error}\n`)}}
process.stdout.write(`${cases.length-failures}/${cases.length} account-cycle checks passed\n`)
if(failures)process.exitCode=1
