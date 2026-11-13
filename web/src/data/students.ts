import type { OnTrackKpi, PaceRule, StudentsResponse } from '../domain';
import { mockStudents } from './mockStudents';

export type FollowUp = {
  id: number;
  studentId: number;
  note: string;
  createdAt: string;
  /** Pulse account id of the author, e.g. "01edu-mentor-12" (set by the server from the session). */
  authorId?: string;
  authorName?: string;
  authorRole?: string;
  /** True when the signed-in person may delete this note (its author or a manager). */
  canDelete?: boolean;
};

type InterventionPayload = {
  id: number;
  studentId: number;
  action: string;
  createdAt: string;
  authorId?: number;
  authorName?: string;
  authorRole?: string;
  canDelete?: boolean;
};

export function usesLiveApi(): boolean {
  return (import.meta.env.VITE_STUDENTS_SOURCE ?? 'api') === 'api';
}

// The session lives in an HttpOnly cookie set by the API, so every request
// sends credentials; the browser never stores identity or role itself.
function apiUrl(path: string): { url: string; credentials: RequestCredentials } {
  const baseUrl = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');
  return { url: `${baseUrl}${path}`, credentials: 'include' };
}

/** Fired when the API answers 401: the session ended, so the app shows the sign-in page. */
export const UNAUTHORIZED_EVENT = 'pulse:unauthorized';

async function readJSON<T>(response: Response): Promise<T> {
  if (response.status === 401) {
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
    throw new Error('UNAUTHENTICATED');
  }
  if (response.status === 403) throw new Error('You do not have access to this.');
  if (!response.ok) throw new Error(`API returned ${response.status}. Please try again.`);
  return await response.json() as T;
}

const DEMO_LATENCY_MS = 320;

// Keep mock and live records behind one adapter so UI components do not need
// source-specific branches or transport details.
function waitForDemoNetwork(): Promise<void> {
  return new Promise(resolve => window.setTimeout(resolve, DEMO_LATENCY_MS));
}

export type SyncStageReport = { ok: boolean; rows: number; applied: number; students: number; error?: string };

/** Secret-free state of the backend's scheduled 01Edu live sync. */
export type SyncStatus = {
  /** live: backend has a 01Edu credential; offline: it has none (last synced data); demo: synthetic data in the browser. */
  mode: 'live' | 'offline' | 'demo';
  credentialMode?: 'jwt' | 'app-token' | 'password';
  role?: string;
  canReadCohort?: boolean;
  intervalSeconds?: number;
  running?: boolean;
  lastSuccessAt?: string;
  lastAttemptAt?: string;
  nextRunAt?: string;
  lastError?: string;
  /** "skipped: another instance is syncing" when another server held the sync lock. */
  lastSkipped?: string;
  lastSkippedAt?: string;
  /** Daily pace snapshot job (runs without a 01Edu credential too). */
  snapshots?: { intervalSeconds: number; running: boolean; lastSuccessAt?: string; lastAttemptAt?: string; nextRunAt?: string; lastError?: string; lastSkipped?: string; students: number };
  lastReport?: { status: 'SUCCESS' | 'LIMITED' | 'FAILED'; cohortWide: boolean; roster?: SyncStageReport; levels: SyncStageReport; records: SyncStageReport; progress: SyncStageReport; snapshots: SyncStageReport };
};

export async function getSyncStatus(): Promise<SyncStatus> {
  if (!usesLiveApi()) return { mode: 'demo' };
  const request = apiUrl('/api/v1/sync/status');
  return readJSON<SyncStatus>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
}

export async function getStudents(): Promise<StudentsResponse> {
  const source = import.meta.env.VITE_STUDENTS_SOURCE ?? 'api';

  if (source === 'mock') {
    await waitForDemoNetwork();
    // Return an API-shaped copy, so screens cannot accidentally mutate fixtures.
    return { ...structuredClone(mockStudents), dataSource: 'demo' };
  }

  if (source === 'api') {
    const program = await getProgram();
    const request = apiUrl(`/api/v1/cohorts/${program.defaultCohortId}/students`);
    const response = await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials });
    return readJSON<StudentsResponse>(response);
  }

  throw new Error(`Unknown VITE_STUDENTS_SOURCE "${source}". Use "mock" or "api".`);
}

function asFollowUp(row: InterventionPayload): FollowUp {
  const authorId = row.authorId && row.authorRole ? `01edu-${row.authorRole}-${row.authorId}` : undefined;
  return { id: row.id, studentId: row.studentId, note: row.action, createdAt: row.createdAt, authorId, authorName: row.authorName || undefined, authorRole: row.authorRole || undefined, canDelete: row.canDelete === true };
}

export async function getFollowUps(): Promise<FollowUp[]> {
  const request = apiUrl('/api/v1/interventions');
  const body = await readJSON<{ data: InterventionPayload[] | null }>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
  return (body.data ?? []).map(asFollowUp);
}

export type ProgressBoard = {
  source: string;
  syncedAt?: string;
  projects: { id: number; name: string; path: string }[];
  students: { studentId: number; login: string; displayName: string; mentorName: string; done: boolean[]; completed: number; total: number }[];
};

export type StaffMember = {
  studentId: number;
  sourceUserId: number;
  login: string;
  displayName: string;
  email: string;
  sourceRole: string;
  pulseRole: 'manager' | 'mentor';
};

export type AcademyIdentity = {
  studentId: number;
  sourceUserId: number;
  login: string;
  displayName: string;
  email: string;
  sourceRole: string;
  pulseRole: 'manager' | 'mentor' | 'student';
};

/** Signs in with the 01Edu login and password. The API verifies them with 01Edu and sets the session cookie. */
export async function signInWithAcademyLogin(login: string, password: string): Promise<AcademyIdentity> {
  const request = apiUrl('/api/v1/session');
  const response = await fetch(request.url, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    credentials: request.credentials,
    body: JSON.stringify({ login, password }),
  });
  if (response.status === 401) throw new Error('INVALID_CREDENTIALS');
  if (response.status === 404) throw new Error('NOT_IN_COHORT');
  if (response.status === 429) throw new Error('TOO_MANY_ATTEMPTS');
  if (!response.ok) throw new Error('SIGNIN_UNAVAILABLE');
  return await response.json() as AcademyIdentity;
}

/** The person behind the current session cookie, or null when signed out. */
export async function getCurrentSession(): Promise<AcademyIdentity | null> {
  const request = apiUrl('/api/v1/session');
  const response = await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials, cache: 'no-store' });
  if (response.status === 401) return null;
  if (!response.ok) throw new Error('SESSION_UNAVAILABLE');
  return await response.json() as AcademyIdentity;
}

/** Ends the server session and clears the cookie. */
export async function signOutSession(): Promise<void> {
  const request = apiUrl('/api/v1/session');
  await fetch(request.url, { method: 'DELETE', credentials: request.credentials }).catch(() => undefined);
}

/** Reads the API's error message so the screen can show why a change failed. */
async function failure(response: Response, fallback: string, forbiddenText = 'You do not have permission to change mentor assignments.'): Promise<Error> {
  if (response.status === 401) {
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
    return new Error('UNAUTHENTICATED');
  }
  if (response.status === 403) return new Error(forbiddenText);
  try {
    const body = await response.json() as { error?: { message?: string } };
    if (body.error?.message) return new Error(body.error.message);
  } catch { /* not JSON */ }
  return new Error(`${fallback} (API returned ${response.status}).`);
}

/** Makes mentorId (the mentor's person id) the student's current mentor. The server keeps the previous one in the history. */
export async function assignMentor(studentId: number, mentorId: number): Promise<void> {
  const request = apiUrl('/api/v1/mentor-assignments');
  const response = await fetch(request.url, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    credentials: request.credentials,
    body: JSON.stringify({ studentId, mentorId }),
  });
  if (!response.ok) throw await failure(response, 'The mentor could not be assigned');
}

/** Ends the student's current mentor assignment (it stays in the history). */
export async function unassignMentor(studentId: number): Promise<void> {
  const request = apiUrl(`/api/v1/mentor-assignments/${encodeURIComponent(String(studentId))}`);
  const response = await fetch(request.url, { method: 'DELETE', headers: { Accept: 'application/json' }, credentials: request.credentials });
  if (!response.ok) throw await failure(response, 'The mentor could not be removed');
}

export async function getStaff(): Promise<{ managers: StaffMember[]; mentors: StaffMember[] }> {
  const request = apiUrl('/api/v1/staff');
  const body = await readJSON<{ managers: StaffMember[] | null; mentors: StaffMember[] | null }>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
  return { managers: body.managers ?? [], mentors: body.mentors ?? [] };
}

export async function getCohortProgress(): Promise<ProgressBoard> {
  const program = await getProgram();
  const request = apiUrl(`/api/v1/cohorts/${program.defaultCohortId}/progress`);
  return readJSON<ProgressBoard>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
}

export async function getStudentProgress(studentId: number): Promise<ProgressBoard> {
  const request = apiUrl(`/api/v1/students/${studentId}/progress`);
  return readJSON<ProgressBoard>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
}

export async function createFollowUp(input: { studentId: number; note: string }): Promise<FollowUp> {
  const request = apiUrl('/api/v1/interventions');
  const body = await readJSON<{ data: InterventionPayload }>(await fetch(request.url, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    credentials: request.credentials,
    body: JSON.stringify({ studentId: input.studentId, action: input.note, reasonCode: 'MENTOR_FOLLOW_UP', type: 'follow-up' }),
  }));
  return asFollowUp(body.data);
}

/** Deletes a follow-up (soft delete on the server). Only its author or a manager may. */
export async function deleteFollowUp(id: number): Promise<void> {
  const request = apiUrl(`/api/v1/interventions/${encodeURIComponent(String(id))}`);
  const response = await fetch(request.url, { method: 'DELETE', headers: { Accept: 'application/json' }, credentials: request.credentials });
  if (response.status === 404) throw new Error('This follow-up was already deleted.');
  if (!response.ok) throw await failure(response, 'The follow-up could not be deleted', 'Only the author or a program manager can delete this follow-up.');
}

/** The curriculum event and cohorts the backend is configured for (ZEROONE_EVENT_ID, PILOT_COHORTS). */
export type Program = {
  eventId: number;
  defaultCohortId: number;
  cohorts: { id: number; name: string }[];
  paceRule?: PaceRule | null;
};

let programRequest: Promise<Program> | null = null;

/** Reads GET /api/v1/program once per page load; the app never hard-codes the event or cohort. */
export function getProgram(): Promise<Program> {
  if (!programRequest) {
    const request = apiUrl('/api/v1/program');
    programRequest = fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials })
      .then(response => readJSON<Program>(response))
      .catch(error => { programRequest = null; throw error; });
  }
  return programRequest;
}

/** GET /api/v1/kpi/on-track: program, cohort and mentor values for a manager; only their own value for a mentor. */
export type OnTrackReport = {
  eventId: number;
  definition: string;
  program: OnTrackKpi | null;
  cohorts: OnTrackKpi[];
  mentors: OnTrackKpi[];
};

export async function getOnTrackKpis(): Promise<OnTrackReport> {
  const request = apiUrl('/api/v1/kpi/on-track');
  const body = await readJSON<OnTrackReport>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
  return { ...body, program: body.program ?? null, cohorts: body.cohorts ?? [], mentors: body.mentors ?? [] };
}
