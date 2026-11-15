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
  const levels = useMemo(
    () =>
      [...new Set(rows.map((row) => row.actualLevel))].sort((a, b) => a - b),
    [rows],
  );
  const initialLevel =
    rows.find((row) => row.studentId === highlightedStudentId)?.actualLevel ??
    levels[0] ??
    null;
  const [selectedLevel, setSelectedLevel] = useState<number | null>(
    initialLevel,
  );
  const visibleRows =
    selectedLevel === null
      ? rows
      : rows.filter((row) => row.actualLevel === selectedLevel);
  const maxCount = Math.max(
    1,
    ...levels.map(
      (level) => rows.filter((row) => row.actualLevel === level).length,
    ),
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
      {levels.length ? (
        <>
          <div
            className="cohort-histogram-chart"
            role="group"
            aria-label={
              ar ? "أعمدة توزيع المستويات" : "Level distribution bars"
            }
          >
            {levels.map((level) => {
              const count = rows.filter(
                (row) => row.actualLevel === level,
              ).length;
              const selected = selectedLevel === level;
              return (
                <button
                  type="button"
                  className={`cohort-level-bar${selected ? " selected" : ""}`}
                  key={level}
                  aria-pressed={selected}
                  aria-label={`${ar ? "المستوى" : "Level"} ${level}: ${count} ${ar ? "طالب" : count === 1 ? "student" : "students"}`}
                  onClick={() => setSelectedLevel(selected ? null : level)}
                >
                  <span className="cohort-level-count">{count}</span>
                  <span className="cohort-level-bar-track">
                    <i
                      style={{
                        height: `${Math.max(8, (count / maxCount) * 100)}%`,
                      }}
                    />
                  </span>
                  <b>{level}</b>
                </button>
              );
            })}
          </div>
          <div className="cohort-level-roster" aria-live="polite">
            <div className="cohort-level-roster-heading">
              <b>
                {selectedLevel === null
                  ? ar
                    ? "كل الطلاب"
                    : "All students"
                  : `${ar ? "المستوى" : "Level"} ${selectedLevel}`}
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
              {visibleRows.map((student) => {
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
