export type ActiveKpiSourceStatus =
  | 'active'
  | 'restricted'
  | 'blocked-long'
  | 'ban'
  | 'left-school'
  | 'away'
  | 'temporary-notice'
  | 'observation';

/** One row of the status_policy table (only the KPI flag is needed here). */
export type StatusPolicy = { recordType: string; excludeFromActiveKpis: boolean };

/** The on-track KPI as the API returns it (GET /api/v1/kpi/on-track). */
export type OnTrackKpi = {
  scope?: 'program' | 'cohort' | 'mentor' | string;
  eventId?: number;
  cohortId?: number;
  mentorId?: number;
  name?: string;
  /** Numerator: measurable GREEN students. */
  onTrack: number;
  /** Denominator: measurable students. */
  measurable: number;
  /** Percentage 0–100; null when there are no measurable students. */
  rate: number | null;
  hasMeasurable: boolean;
  total: number;
  excludedByPolicy: number;
  statusCounts: Record<string, number>;
  measurableCounts: { GREEN: number; YELLOW: number; RED: number };
};

export type OnTrackState = { status: 'loading' | 'error' | 'ready'; error?: string };

export const measurablePaceStatuses: readonly string[];
export function computeOnTrack(students: { paceStatus: string; sourceStatus: string }[], statusPolicy: StatusPolicy[]): OnTrackKpi;
export function describeOnTrack(state: OnTrackState, kpi: OnTrackKpi | null | undefined): { value: string; foot: string; tone: 'loading' | 'error' | 'empty' | 'ready' };
