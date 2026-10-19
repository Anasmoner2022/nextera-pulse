export type ActiveKpiSourceStatus =
  | 'active'
  | 'restricted'
  | 'blocked-long'
  | 'ban'
  | 'left-school'
  | 'away'
  | 'temporary-notice'
  | 'observation';

export function isInActiveKpiPopulation(student: { sourceStatus: ActiveKpiSourceStatus; }): boolean;
