import type { StudentsResponse } from '../domain';
import { mockStudents } from './mockStudents';

const DEMO_LATENCY_MS = 320;

function waitForDemoNetwork(): Promise<void> {
  return new Promise(resolve => window.setTimeout(resolve, DEMO_LATENCY_MS));
}

export async function getStudents(): Promise<StudentsResponse> {
  const source = import.meta.env.VITE_STUDENTS_SOURCE ?? 'mock';

  if (source === 'mock') {
    await waitForDemoNetwork();
    // Return an API-shaped copy, so screens cannot accidentally mutate fixtures.
    return structuredClone(mockStudents);
  }

  if (source === 'api') {
    const baseUrl = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');
    if (!baseUrl) throw new Error('VITE_API_BASE_URL is required when VITE_STUDENTS_SOURCE=api.');
    const response = await fetch(`${baseUrl}/api/v1/cohorts/97/students`, {
      headers: { Accept: 'application/json' },
      credentials: 'include',
    });
    if (!response.ok) throw new Error(`Students API returned ${response.status}. Please try again.`);
    return await response.json() as StudentsResponse;
  }

  throw new Error(`Unknown VITE_STUDENTS_SOURCE "${source}". Use "mock" or "api".`);
}
