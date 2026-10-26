export type StoredAccount = { id: string; passwordHash?: string; };
export function persistProfileAccount<T extends StoredAccount>(storage: Storage, account: T, key?: string): T[];
export function readStoredAccounts<T extends StoredAccount = StoredAccount>(storage: Storage, key?: string): T[];
export function hashAccountPassword(password: string): Promise<string>;
export function authenticateStoredAccount<T extends StoredAccount = StoredAccount>(storage: Storage, email: string, password: string): Promise<Omit<T, 'passwordHash'> | null>;
