import { useMemo, useState } from "react";
import type { StudentPaceRow } from "../../domain";
import type { PulseAccount } from "../../identity";
import { findAssignedMentor } from "../../studentAccountLink.js";
import { statusNames } from "../config";
import type { Language } from "../config";

// Group students into level buckets; names remain in the accessible roster below.
export function CohortLevelHistogram({
  rows,
  accounts,
  mentors,
  lang,
  highlightedStudentId,
  onSelectStudent,
}: {
  rows: StudentPaceRow[];
  accounts: PulseAccount[];
  mentors: PulseAccount[];
  lang: Language;
  highlightedStudentId?: number;
  onSelectStudent?: (student: StudentPaceRow) => void;
}) {
  const ar = lang === "ar";
  const buckets = useMemo(() => {
    const levels = [...new Set(rows.map((row) => row.actualLevel))].sort((a, b) => a - b);
    const min = levels[0] ?? 0;
    const max = levels.at(-1) ?? min;
    const step = levels.length > 16 ? Math.max(1, Math.ceil((max - min + 1) / 16)) : 1;
    const grouped = new Map<string, { label: string; rows: StudentPaceRow[] }>();
    for (const row of rows) {
      const start = Math.floor((row.actualLevel - min) / step) * step + min;
      const end = start + step - 1;
      const key = `${start}:${end}`;
      const bucket = grouped.get(key) ?? { label: step === 1 ? String(start) : `${start}–${end}`, rows: [] };
      bucket.rows.push(row);
      grouped.set(key, bucket);
    }
    return [...grouped.entries()].map(([key, bucket]) => ({ key, ...bucket }));
  }, [rows]);
  const highlightedLevel = rows.find((row) => row.studentId === highlightedStudentId)?.actualLevel;
  const [selectedBucket, setSelectedBucket] = useState<string | null>(() => {
    const bucket = buckets.find((item) => item.rows.some((row) => row.actualLevel === highlightedLevel));
    return bucket?.key ?? buckets[0]?.key ?? null;
  });
  const currentBucket = buckets.find((bucket) => bucket.key === selectedBucket) ?? buckets[0];
  const visibleRows = currentBucket?.rows ?? rows;
  const [showAll, setShowAll] = useState(false);
  const shownRows = showAll || visibleRows.length <= 25 ? visibleRows : visibleRows.slice(0, 25);
  const maxCount = Math.max(
    1,
    ...buckets.map((bucket) => bucket.rows.length),
  );

  return (
    <section
      className="cohort-histogram"
      aria-label={ar ? "توزيع مستويات الطلاب" : "Student level distribution"}
    >
      <header>
        <div>
          <span className="eyebrow">
            {ar ? "مقارنة الكوهورت" : "COHORT COMPARISON"}
          </span>
          <h2>
            {ar ? "توزيع الطلاب حسب المستوى" : "Students by current level"}
          </h2>
          <p>
            {ar
              ? "اختر عمودًا لعرض أسماء الطلاب والمنتور وحالة التقدم."
              : "Select a bar to see the students, their mentor, and pace status."}
          </p>
        </div>
        <span className="cohort-histogram-total">
          {rows.length}{" "}
          {ar ? "طالب" : rows.length === 1 ? "student" : "students"}
        </span>
      </header>
      {buckets.length ? (
        <>
          <div
            className="cohort-histogram-chart"
            role="group"
            aria-label={
              ar ? "أعمدة توزيع المستويات" : "Level distribution bars"
            }
          >
            {buckets.map((bucket) => {
              const count = bucket.rows.length;
              const selected = currentBucket?.key === bucket.key;
              return (
                <button
                  type="button"
                  className={`cohort-level-bar${selected ? " selected" : ""}`}
                  key={bucket.key}
                  aria-pressed={selected}
                  aria-label={`${ar ? "المستوى" : "Level"} ${bucket.label}: ${count} ${ar ? "طالب" : count === 1 ? "student" : "students"}`}
                  onClick={() => { setSelectedBucket(bucket.key); setShowAll(false); }}
                >
                  <span className="cohort-level-count">{count}</span>
                  <span className="cohort-level-bar-track">
                    <i
                      style={{
                        height: `${Math.max(8, (count / maxCount) * 100)}%`,
                      }}
                    />
                  </span>
                  <b>{bucket.label}</b>
                </button>
              );
            })}
          </div>
          <div className="cohort-level-roster" aria-live="polite">
            <div className="cohort-level-roster-heading">
              <b>
                {currentBucket ? `${ar ? "المستوى" : "Level"} ${currentBucket.label}` : ar ? "كل الطلاب" : "All students"}
              </b>
              <span>
                {visibleRows.length}{" "}
                {ar
                  ? "طالب"
                  : visibleRows.length === 1
                    ? "student"
                    : "students"}
              </span>
            </div>
            <div className="cohort-level-roster-grid">
              {shownRows.map((student) => {
                const mentor = findAssignedMentor(student, accounts, mentors);
                const content = (
                  <>
                    <span className="cohort-level-student-name">
                      <b>{student.displayName || student.login}</b>
                      <small>@{student.login}</small>
                    </span>
                    <span className="cohort-level-mentor">
                      {mentor?.name ?? (ar ? "غير مسند" : "Unassigned")}
                    </span>
                    <span
                      className={`status-badge ${student.paceStatus.toLowerCase()}`}
                    >
                      <i />
                      {statusNames[lang][student.paceStatus]}
                    </span>
                  </>
                );
                return onSelectStudent ? (
                  <button
                    type="button"
                    className={`cohort-level-student${student.studentId === highlightedStudentId ? " highlighted" : ""}`}
                    key={student.studentId}
                    onClick={() => onSelectStudent(student)}
                  >
                    {content}
                  </button>
                ) : (
                  <article
                    className={`cohort-level-student${student.studentId === highlightedStudentId ? " highlighted" : ""}`}
                    key={student.studentId}
                  >
                    {content}
                  </article>
                );
              })}
            </div>
            {visibleRows.length > 25 && <button className="cohort-level-view-all" type="button" onClick={() => setShowAll((value) => !value)}>{showAll ? (ar ? 'عرض أول 25' : 'Show first 25') : `${ar ? 'عرض الكل' : 'View all'} ${visibleRows.length}`}</button>}
          </div>
        </>
      ) : (
        <p className="analysis-empty">
          {ar
            ? "لا توجد بيانات مستويات متاحة."
            : "No level data is available yet."}
        </p>
      )}
    </section>
  );
}
