import { readStoredAccounts } from '../accountPersistence.js';
import type { PulseAccount } from './types';

const accountKey = 'pulse-accounts';
function cleanAccount(account: PulseAccount): PulseAccount {
  const { bio: _bio, posts: _posts, ...saved } = account as PulseAccount & { bio?: unknown; posts?: unknown; };
  return { ...saved, permissions: saved.permissions.filter(permission => String(permission) !== 'community') };
}
export function readAccounts(): PulseAccount[] { return readStoredAccounts<PulseAccount>(localStorage, accountKey).map(cleanAccount); }
export function writeAccounts(accounts: PulseAccount[]): void { localStorage.setItem(accountKey, JSON.stringify(accounts.map(cleanAccount))); }
