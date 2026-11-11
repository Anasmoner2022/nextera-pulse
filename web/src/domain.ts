export type PaceStatus =
  | 'GREEN'
  | 'YELLOW'
  | 'RED'
  | 'PAUSED'
  | 'INACTIVE'
  | 'NOT_MEASURABLE';

import { isInActiveKpiPopulation } from './studentMetrics.mjs';
export { isInActiveKpiPopulation };
import type { ActiveKpiSourceStatus } from './studentMetrics.mjs';

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

export type StudentsResponse = {
  cohortName: string;
  eventId: number;
  snapshotDate: string;
  students: StudentPaceRow[];
};
