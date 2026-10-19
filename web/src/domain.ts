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
  enrolledAt: string;
  activeAcademicDays: number;
  curriculumMonth: number;
  actualLevel: number;
  minimumLevel: number | null;
  expectedLevel: number | null;
  checkpointTarget: number | null;
  gapToMinimum: number | null;
  gapToExpected: number | null;
  paceStatus: PaceStatus;
  sourceStatus: AcademyStatus;
  healthReasonCodes: string[];
};

export type StudentsResponse = {
  cohortName: string;
  eventId: number;
  snapshotDate: string;
  students: StudentPaceRow[];
};
