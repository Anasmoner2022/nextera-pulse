import { useEffect, useMemo, useState } from 'react';
import { computeOnTrack } from '../domain';
import type { OnTrackKpi, OnTrackState, StudentPaceRow } from '../domain';
import { getOnTrackKpis } from '../data/students';
import { mockStatusPolicy } from '../data/mockStudents';
import type { PulseAccount } from '../identity';
import { normalizeLogin } from './config';

/** The on-track KPI for every scope the dashboard shows, with its load state. */
export type OnTrackKpis = OnTrackState & {
  program: OnTrackKpi | null;
  /** Keyed by the mentor's Pulse account id ("01edu-mentor-12" live, local ids in demo). */
  mentors: Record<string, OnTrackKpi>;
};

/**
 * Live: reads GET /api/v1/kpi/on-track (the server computes the rate from the
 * status_policy table). It reloads when refreshKey changes (new sync, new
 * assignments). Demo: the same formula on mock rows and the mock policy.
 */
export function useOnTrackKpis({ live, rows, mentors, refreshKey }: { live: boolean; rows: StudentPaceRow[]; mentors: PulseAccount[]; refreshKey: string }): OnTrackKpis {
  const [remote, setRemote] = useState<OnTrackKpis>({ status: 'loading', program: null, mentors: {} });
  useEffect(() => {
    if (!live) return;
    let active = true;
    setRemote(current => ({ ...current, status: 'loading', error: undefined }));
    getOnTrackKpis()
      .then(report => {
        if (!active) return;
        const byMentor: Record<string, OnTrackKpi> = {};
        for (const kpi of report.mentors) if (kpi.mentorId) byMentor[`01edu-mentor-${kpi.mentorId}`] = kpi;
        setRemote({ status: 'ready', program: report.program, mentors: byMentor });
      })
      .catch(error => { if (active) setRemote({ status: 'error', error: error instanceof Error ? error.message : 'Unknown error', program: null, mentors: {} }); });
    return () => { active = false; };
  }, [live, refreshKey]);

  const demo = useMemo<OnTrackKpis | null>(() => {
    if (live) return null;
    const byMentor: Record<string, OnTrackKpi> = {};
    for (const mentor of mentors) {
      const logins = new Set((mentor.assignedStudentLogins ?? []).map(normalizeLogin));
      byMentor[mentor.id] = { ...computeOnTrack(rows.filter(row => logins.has(normalizeLogin(row.login))), mockStatusPolicy), scope: 'mentor', name: mentor.name };
    }
    return { status: 'ready', program: { ...computeOnTrack(rows, mockStatusPolicy), scope: 'program' }, mentors: byMentor };
  }, [live, rows, mentors]);

  return demo ?? remote;
}
