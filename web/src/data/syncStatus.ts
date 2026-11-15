import { useSyncExternalStore } from 'react';
import { getSyncStatus, type SyncStatus } from './students';

// One shared poller for GET /api/v1/sync/status, so every screen agrees on
// whether it is showing live 01Edu data, the last synced data (offline), or demo data.
const POLL_MS = 60_000;

type Snapshot = { status: SyncStatus | null; error: string };

let snapshot: Snapshot = { status: null, error: '' };
const listeners = new Set<() => void>();
let timer: number | undefined;

function emit(next: Snapshot) {
  snapshot = next;
  listeners.forEach(listener => listener());
}

async function poll() {
  try {
    emit({ status: await getSyncStatus(), error: '' });
  } catch (e) {
    emit({ status: snapshot.status, error: e instanceof Error ? e.message : 'Sync status unavailable' });
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  if (listeners.size === 1) {
    void poll();
    timer = window.setInterval(() => { void poll(); }, POLL_MS);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== undefined) {
      window.clearInterval(timer);
      timer = undefined;
    }
  };
}

export function useSyncStatus(): Snapshot {
  return useSyncExternalStore(subscribe, () => snapshot, () => snapshot);
}

/** Human label for "how fresh is this data", e.g. "4 min ago". */
export function timeAgo(iso: string | undefined, now: Date = new Date()): string {
  if (!iso) return '';
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return '';
  const minutes = Math.max(0, Math.round((now.getTime() - then) / 60_000));
  if (minutes < 1) return 'just now';
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours} h ago`;
  return new Date(iso).toLocaleDateString();
}
