export type PaceStatus =
  | 'GREEN'
  | 'YELLOW'
  | 'RED'
  | 'PAUSED'
  | 'INACTIVE'
  | 'NOT_MEASURABLE';

import { computeOnTrack, describeOnTrack } from './studentMetrics.mjs';
export { computeOnTrack, describeOnTrack };
import type { ActiveKpiSourceStatus } from './studentMetrics.mjs';
export type { OnTrackKpi, OnTrackState, StatusPolicy } from './studentMetrics.mjs';

export type AcademyStatus = ActiveKpiSourceStatus;

export type StudentPaceRow = {
  studentId: number;
  login: string;
  displayName: string | null;
  email?: string | null;
  age?: number | null;
  phone?: string | null;
  enrolledAt: string;
  activeAcademicDays: number;
  curriculumMonth: number;
  /** Current round within the curriculum month; supplied by the API when available. */
  curriculumRound?: number | null;
  actualLevel: number;
  minimumLevel: number | null;
  expectedLevel: number | null;
  checkpointTarget: number | null;
  gapToMinimum: number | null;
  gapToExpected: number | null;
  paceStatus: PaceStatus;
  sourceStatus: AcademyStatus;
  healthReasonCodes: string[];
  mentorId?: number;
  mentorName?: string;
};

/** The monthly-target version behind the pace statuses (from the API). */
export type PaceRule = { eventId: number; version: string; status: 'draft' | 'approved' | 'retired' | 'missing' | string; approved: boolean };

export type StudentsResponse = {
  cohortId?: number;
  cohortName: string;
  eventId: number;
  paceRule?: PaceRule;
  snapshotDate: string;
  /** live: the backend syncs 01Edu; offline: no 01Edu credential on the backend (last synced data); demo: synthetic. */
  dataSource?: 'live' | 'offline' | 'demo';
  /** Time of the last successful 01Edu sync. */
  syncedAt?: string;
  students: StudentPaceRow[];
};
