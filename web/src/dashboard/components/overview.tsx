import { useState } from "react";
import { describeOnTrack, isInActiveKpiPopulation } from "../../domain";
import type { PaceStatus, StudentPaceRow } from "../../domain";
import { ProfileAvatar } from "../../identity";
import type { PulseAccount } from "../../identity";
import { StudentAvatar } from "./students";
import { ProjectProgress } from "./project-progress";
import { CohortLevelHistogram } from "./CohortLevelHistogram";
import { normalizeLogin, statusNames, text } from "../config";
import type { Intervention, Language } from "../config";
import type { OnTrackKpis } from "../onTrack";

// Keep a single presentation primitive for the high-level workspace metrics.
export function Stat({
  icon,
  label,
  value,
  foot,
  color,
}: {
  icon: string;
  label: string;
  value: number | string;
  foot: string;
  color: string;
}) {
  return (
    <article className="stat-card">
      <div className={`stat-icon ${color}`}>{icon}</div>
      <span className="stat-label">{label}</span>
      <strong>{value}</strong>
      <small>{foot}</small>
      <span className="stat-spark">⌁</span>
    </article>
  );
}

// Summarize mentor assignments and cohort health for the program manager.
export function ManagerOverview({
  mentors,
  rows,
  interventions,
  lang,
  kpis,
  onOpenTeam,
  onOpenMentor,
}: {
  mentors: PulseAccount[];
  rows: StudentPaceRow[];
  interventions: Intervention[];
  lang: Language;
  kpis: OnTrackKpis;
  onOpenTeam?: () => void;
  onOpenMentor?: (mentor: PulseAccount) => void;
}) {
  const ar = lang === "ar";
  const assigned = new Set(
    mentors
      .flatMap((mentor) => mentor.assignedStudentLogins ?? [])
      .map(normalizeLogin),
  );
  const unassigned = rows.filter(
    (row) => !assigned.has(normalizeLogin(row.login)),
  );
  const programRate = describeOnTrack(kpis, kpis.program);
  const metrics = [
    {
      icon: "◎",
      label: ar ? "على المسار" : "On-track rate",
      value: programRate.value,
      note: programRate.foot,
      tone: programRate.tone === "error" ? "amber" : "green",
    },
    {
      icon: "✦",
      label: ar ? "المنتورز" : "Mentors",
      value: mentors.length,
      note: ar ? "حسابات الفريق" : "Team accounts",
      tone: "mint",
    },
    {
      icon: "♙",
      label: ar ? "طلاب ضمن المؤشرات" : "Active KPI students",
      value: rows.filter(isInActiveKpiPopulation).length,
      note: ar ? `من ${rows.length} سجلًا` : `of ${rows.length} cohort records`,
      tone: "blue",
    },
    {
      icon: "＋",
      label: ar ? "طلاب دون منتور" : "Unassigned students",
      value: unassigned.length,
      note: ar ? "يحتاجون توزيعًا" : "Need an assignment",
      tone: unassigned.length ? "amber" : "green",
    },
  ];
  return (
    <section className="manager-overview">
      <div className="manager-overview-intro">
        <div>
          <h2>
            {ar
              ? `${rows.length} طالبًا يتابعهم ${mentors.length} منتور.`
              : `${rows.length} students across ${mentors.length} mentors.`}
          </h2>
          <p>
            {ar
              ? "تابع توزيع الطلاب وحمل المنتورز ومؤشرات التقدم من مكان واحد."
              : "Review student assignments, mentor workload, and progress in one place."}
          </p>
        </div>
        {onOpenTeam && (
          <button className="setting-action" onClick={onOpenTeam}>
            {ar ? "إدارة التوزيع" : "Manage assignments"} ↗
          </button>
        )}
      </div>
      <div className="manager-summary-grid">
        {metrics.map((metric) => (
          <article
            className={`manager-metric ${metric.tone}`}
            key={metric.label}
          >
            <span className="manager-metric-icon">{metric.icon}</span>
            <div>
              <small>{metric.label}</small>
              <strong>{metric.value}</strong>
              <span>{metric.note}</span>
            </div>
          </article>
        ))}
      </div>
      {unassigned.length > 0 && (
        <div className="assignment-alert">
          <span>
            <b>
              {ar
                ? `${unassigned.length} طلاب محتاجين يتوزعوا`
                : `${unassigned.length} students need an assignment`}
            </b>
            <small>
              {ar
                ? "وزّعهم على منتور عشان يظهروا في حسابه."
                : "Assign them to a mentor so they appear in that mentor’s workspace."}
            </small>
          </span>
          {onOpenTeam && (
            <button className="setting-action" onClick={onOpenTeam}>
              {ar ? "وزّع الطلاب" : "Assign students"} ↗
            </button>
          )}
        </div>
      )}
      <div className="manager-section-heading">
        <div>
          <h3>{ar ? "أداء المنتورز" : "Mentor performance"}</h3>
          <p>
            {ar
              ? "حجم المجموعة والتقدم والمتابعات لكل منتور."
              : "Workload, student pace, and follow-ups for each mentor."}
          </p>
        </div>
        <span>
          {mentors.length}{" "}
          {ar ? "منتور" : mentors.length === 1 ? "mentor" : "mentors"}
        </span>
      </div>
      <div className="manager-mentor-grid">
        {mentors.map((mentor) => {
          const loginSet = new Set(
            (mentor.assignedStudentLogins ?? []).map(normalizeLogin),
          );
          const assignedRows = rows.filter((row) =>
            loginSet.has(normalizeLogin(row.login)),
          );
          const mentorKpi = kpis.mentors[mentor.id];
          const mentorRate = describeOnTrack(kpis, mentorKpi);
          const onTrack = mentorKpi?.measurableCounts.GREEN ?? 0;
          const attention =
            (mentorKpi?.measurableCounts.YELLOW ?? 0) +
            (mentorKpi?.measurableCounts.RED ?? 0);
          const followUps = interventions.filter((item) =>
            assignedRows.some((row) => row.studentId === item.studentId),
          ).length;
          const onTrackRate = mentorKpi?.rate ?? 0;
          return (
            <article className="manager-mentor-card" key={mentor.id}>
              <div className="manager-mentor-head">
                <ProfileAvatar account={mentor} size="medium" />
                <div>
                  <b>{mentor.name}</b>
                  <small>{mentor.email}</small>
                </div>
                <span className="manager-track-rate">
                  {mentorRate.value}<small>{ar ? "على المسار" : "on track"}</small>
                </span>
              </div>
              <div className="manager-cohort-rate">
                <div>
                  <span>
                    {ar
                      ? "معدل الطلاب النشطين على المسار"
                      : "Active students on track"}
                  </span>
                  <b>
                    {kpis.status === "ready"
                      ? `${onTrack} / ${mentorKpi?.measurable ?? 0}`
                      : "—"}
                  </b>
                </div>
                <div
                  className="manager-rate-track"
                  role="img"
                  aria-label={`${onTrackRate}% of active students are on track`}
                >
                  <i style={{ width: `${onTrackRate}%` }} />
                </div>
              </div>
              <div className="manager-mentor-stats">
                <div>
                  <small>{ar ? "طلاب على المسار" : "On track"}</small>
                  <b>{onTrack}</b>
                </div>
                <div>
                  <small>{ar ? "محتاجين متابعة" : "Need attention"}</small>
                  <b>{attention}</b>
                </div>
                <div>
                  <small>{ar ? "متابعات" : "Follow-ups"}</small>
                  <b>{followUps}</b>
                </div>
              </div>
              {onOpenMentor && (
                <button
                  type="button"
                  className="mentor-open-workspace"
                  onClick={() => onOpenMentor(mentor)}
                >
                  {ar ? "فتح مساحة المنتور" : "Open mentor workspace"}{" "}
                  <span aria-hidden="true">→</span>
                </button>
              )}
            </article>
          );
        })}
        {mentors.length === 0 && (
          <div className="empty-state">
            <b>
              {ar
                ? "لا توجد حسابات منتورز حتى الآن"
                : "No mentor accounts yet."}
            </b>
          </div>
        )}
      </div>
      <p className="manager-footnote">
        {ar
          ? "معدل على المسار محسوب من الطلاب ذوي بيانات منهج قابلة للقياس. المتوقفون وغير النشطين ومن لا يملكون جدولًا زمنيًا مستبعدون."
          : "On-track rates use active students with measurable academy records. Paused, inactive, and no-timeline records are excluded from the denominator."}
      </p>
    </section>
  );
}

// Render a scalable histogram and keep names in the roster below the chart.
function CohortPositionCurve({
  rows,
  lang,
  onSelect,
  accounts,
  mentors,
}: {
  rows: StudentPaceRow[];
  lang: Language;
  onSelect: (student: StudentPaceRow) => void;
  accounts: PulseAccount[];
  mentors: PulseAccount[];
}) {
  return <CohortLevelHistogram rows={rows} accounts={accounts} mentors={mentors} lang={lang} onSelectStudent={onSelect} />;
}
// Combine pace, project completion, and follow-up summaries for the active cohort scope.
export function MentorAnalytics({
  rows,
  interventions,
  lang,
  onSelect,
  scope = "assigned",
  comparisonRows,
  accounts = [],
  mentors = [],
  includeProjects = true,
}: {
  rows: StudentPaceRow[];
  interventions: Intervention[];
  lang: Language;
  onSelect: (student: StudentPaceRow) => void;
  scope?: "assigned" | "program";
  comparisonRows?: StudentPaceRow[];
  accounts?: PulseAccount[];
  mentors?: PulseAccount[];
  includeProjects?: boolean;
}) {
  const ar = lang === "ar";
  const [selectedStatus, setSelectedStatus] = useState<PaceStatus | null>(null);
  const [selectedMonth, setSelectedMonth] = useState<number | null>(null);
  const [levelFocus, setLevelFocus] = useState<"current" | "expected" | "gap">(
    "gap",
  );
  const [selectedAuthor, setSelectedAuthor] = useState<string | null>(null);
  const chartColors: Record<PaceStatus, string> = {
    GREEN: "#36b487",
    YELLOW: "#e6b64f",
    RED: "#e47c70",
    PAUSED: "#85959c",
    INACTIVE: "#b5bdc1",
    NOT_MEASURABLE: "#78aebe",
  };
  // Keep pace charts aligned with the academy's active KPI population.
  const paceRows = rows.filter(isInActiveKpiPopulation);
  const activeStatuses: PaceStatus[] = ["GREEN", "YELLOW", "RED"];
  let chartCursor = 0;
  const statusSlices = activeStatuses.map((status) => {
    const value = paceRows.filter((row) => row.paceStatus === status).length;
    const start = chartCursor;
    chartCursor += paceRows.length ? (value / paceRows.length) * 100 : 0;
    return {
      status,
      value,
      start,
      end: chartCursor,
      color: chartColors[status],
    };
  });
  const statusChart = paceRows.length
    ? `conic-gradient(${statusSlices.map((slice) => `${slice.color} ${slice.start}% ${slice.end}%`).join(",")})`
    : "conic-gradient(#e9efed 0% 100%)";
  const monthCounts = new Map<number, number>();
  rows.forEach((row) =>
    monthCounts.set(
      row.curriculumMonth,
      (monthCounts.get(row.curriculumMonth) ?? 0) + 1,
    ),
  );
  const monthRows = [...monthCounts.entries()].sort(([a], [b]) => a - b);
  const maxMonthCount = Math.max(1, ...monthRows.map(([, count]) => count));
  const measurableRows = rows.filter((row) => row.expectedLevel !== null);
  const avgLevel = (values: number[]) =>
    values.length
      ? values.reduce((sum, value) => sum + value, 0) / values.length
      : 0;
  const actualAverage = avgLevel(measurableRows.map((row) => row.actualLevel));
  const expectedAverage = avgLevel(
    measurableRows.map((row) => row.expectedLevel as number),
  );
  const gapRows = measurableRows.filter((row) => row.gapToExpected !== null);
  const averageGap = avgLevel(
    gapRows.map((row) => row.gapToExpected as number),
  );
  const levelScale = Math.max(
    1,
    ...measurableRows.flatMap((row) => [
      row.actualLevel,
      row.expectedLevel ?? 0,
    ]),
  );
  const followUpAuthors = [
    ...interventions
      .reduce((counts, item) => {
        const name =
          item.authorName ||
          item.authorId ||
          (ar ? "عضو الفريق" : "Team member");
        const current = counts.get(name) ?? {
          count: 0,
          role: item.authorRole || (ar ? "عضو الفريق" : "Team member"),
        };
        current.count += 1;
        counts.set(name, current);
        return counts;
      }, new Map<string, { count: number; role: string }>())
      .entries(),
  ]
    .map(([name, details]) => ({ name, ...details }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 5);
  const maxAuthorCount = Math.max(
    1,
    ...followUpAuthors.map((author) => author.count),
  );
  const levelFocusRows = [...measurableRows]
    .sort((a, b) =>
      levelFocus === "current"
        ? a.actualLevel - b.actualLevel
        : levelFocus === "expected"
          ? (a.expectedLevel ?? 0) - (b.expectedLevel ?? 0)
          : (a.gapToExpected ?? 0) - (b.gapToExpected ?? 0),
    )
    .slice(0, 4);
  const selectedAuthorItems = selectedAuthor
    ? interventions
        .filter(
          (item) =>
            (item.authorName ||
              item.authorId ||
              (ar ? "عضو الفريق" : "Team member")) === selectedAuthor,
        )
        .slice()
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
        .slice(0, 4)
    : [];

  return (
    <section className="mentor-analytics-section">
      <div className="manager-section-heading analysis-heading">
        <div>
          <span className="eyebrow">
            {scope === "program"
              ? ar
                ? "تحليلات البرنامج"
                : "PROGRAM ANALYTICS"
              : ar
                ? "تحليلات طلابك"
                : "STUDENT ANALYTICS"}
          </span>
          <h3>
            {ar
              ? "صورة أوضح عن تقدم المجموعة"
              : scope === "program"
                ? "A closer look at the full cohort"
                : "A closer look at your students"}
          </h3>
          <p>
            {ar
              ? "التقدم، الموقع في المنهج، المستوى، والمتابعات المسجلة."
              : `Pace, curriculum position, levels, and ${scope === "program" ? "team-wide" : "your"} follow-ups.`}
          </p>
        </div>
        <span>
          {rows.length}{" "}
          {ar
            ? scope === "program"
              ? "طالب في البرنامج"
              : "طالب مسند"
            : scope === "program"
              ? rows.length === 1
                ? "student in program"
                : "students in program"
              : rows.length === 1
                ? "assigned student"
                : "assigned students"}
        </span>
      </div>
      <CohortPositionCurve
        rows={comparisonRows ?? rows}
        lang={lang}
        onSelect={onSelect}
        accounts={accounts}
        mentors={mentors}
      />
      <div className="manager-analysis-grid">
        <article className="analysis-card level-analysis">
          <div className="analysis-card-heading">
            <div>
              <small>{ar ? "المستوى والأهداف" : "LEVEL CHECK"}</small>
              <h4>
                {ar
                  ? "المستوى الحالي مقارنة بالمتوقع"
                  : "Current vs expected level"}
              </h4>
            </div>
            <span>
              {measurableRows.length} {ar ? "قابل للقياس" : "measurable"}
            </span>
          </div>
          {measurableRows.length ? (
            <>
              <div className="level-average-pair">
                <button
                  type="button"
                  aria-pressed={levelFocus === "current"}
                  className={levelFocus === "current" ? "selected" : ""}
                  onClick={() => setLevelFocus("current")}
                >
                  <small>
                    {ar ? "متوسط المستوى الحالي" : "Average current"}
                  </small>
                  <b>{actualAverage.toFixed(1)}</b>
                </button>
                <button
                  type="button"
                  aria-pressed={levelFocus === "expected"}
                  className={levelFocus === "expected" ? "selected" : ""}
                  onClick={() => setLevelFocus("expected")}
                >
                  <small>
                    {ar ? "متوسط المستوى المتوقع" : "Average expected"}
                  </small>
                  <b>{expectedAverage.toFixed(1)}</b>
                </button>
                <button
                  type="button"
                  aria-pressed={levelFocus === "gap"}
                  className={levelFocus === "gap" ? "selected" : ""}
                  onClick={() => setLevelFocus("gap")}
                >
                  <small>{ar ? "متوسط الفارق" : "Average gap"}</small>
                  <b
                    className={averageGap < 0 ? "gap-negative" : "gap-positive"}
                  >
                    {averageGap > 0 ? "+" : ""}
                    {averageGap.toFixed(1)}
                  </b>
                </button>
              </div>
              <div className="level-bars">
                <div>
                  <span>{ar ? "المستوى الحالي" : "Current level"}</span>
                  <div>
                    <i
                      className="actual"
                      style={{
                        width: `${(actualAverage / levelScale) * 100}%`,
                      }}
                    />
                  </div>
                </div>
                <div>
                  <span>{ar ? "المستوى المتوقع" : "Expected level"}</span>
                  <div>
                    <i
                      className="expected"
                      style={{
                        width: `${(expectedAverage / levelScale) * 100}%`,
                      }}
                    />
                  </div>
                </div>
              </div>
              <small className="analysis-footnote">
                {ar
                  ? "اضغط على المتوسط لاستكشاف الطلاب."
                  : "Select an average to explore students."}
              </small>
              <div className="analysis-drilldown">
                {levelFocusRows.map((student) => (
                  <button
                    type="button"
                    key={student.studentId}
                    onClick={() => onSelect(student)}
                  >
                    <span>
                      <b>{student.displayName || student.login}</b>
                      <small>@{student.login}</small>
                    </span>
                    <strong>
                      {levelFocus === "current"
                        ? `${ar ? "حالي" : "Current"} ${student.actualLevel}`
                        : levelFocus === "expected"
                          ? `${ar ? "متوقع" : "Expected"} ${student.expectedLevel}`
                          : `${student.gapToExpected! > 0 ? "+" : ""}${student.gapToExpected}`}
                    </strong>
                  </button>
                ))}
              </div>
            </>
          ) : (
            <p className="analysis-empty">
              {ar
                ? "لا توجد مستويات متوقعة للمقارنة بعد."
                : "No expected levels available for comparison yet."}
            </p>
          )}
        </article>
        <article className="analysis-card followup-analysis">
          <div className="analysis-card-heading">
            <div>
              <small>{ar ? "نشاط المتابعة" : "FOLLOW-UP ACTIVITY"}</small>
              <h4>{ar ? "المتابعات حسب الكاتب" : "Follow-ups by author"}</h4>
            </div>
            <span>
              {interventions.length} {ar ? "إجمالي" : "total"}
            </span>
          </div>
          {followUpAuthors.length ? (
            <>
              <div className="author-chart">
                {followUpAuthors.map((author, index) => (
                  <button
                    type="button"
                    aria-pressed={selectedAuthor === author.name}
                    className={`author-chart-row${selectedAuthor === author.name ? " selected" : ""}`}
                    key={author.name}
                    onClick={() =>
                      setSelectedAuthor(
                        selectedAuthor === author.name ? null : author.name,
                      )
                    }
                  >
                    <span className={`author-chart-rank rank-${index + 1}`}>
                      {index + 1}
                    </span>
                    <span className="author-chart-main">
                      <span>
                        <b>{author.name}</b>
                        <small>{author.role}</small>
                      </span>
                      <span className="author-chart-track">
                        <i
                          style={{
                            width: `${(author.count / maxAuthorCount) * 100}%`,
                          }}
                        />
                      </span>
                    </span>
                    <strong>{author.count}</strong>
                  </button>
                ))}
              </div>
              {selectedAuthor && (
                <div className="analysis-drilldown">
                  {selectedAuthorItems.map((item) => {
                    const student = rows.find(
                      (row) => row.studentId === item.studentId,
                    );
                    return (
                      <button
                        type="button"
                        key={item.id}
                        onClick={() => student && onSelect(student)}
                        disabled={!student}
                      >
                        <span>
                          <b>
                            {student?.displayName || (ar ? "طالب" : "Student")}
                          </b>
                          <small>{item.note}</small>
                        </span>
                        <time>
                          {new Date(item.createdAt).toLocaleDateString(
                            ar ? "ar-EG" : "en-GB",
                            { day: "numeric", month: "short" },
                          )}
                        </time>
                      </button>
                    );
                  })}
                </div>
              )}
            </>
          ) : (
            <p className="analysis-empty">
              {ar
                ? "ستظهر المتابعات هنا عند تسجيلها."
                : "Recorded follow-ups will appear here."}
            </p>
          )}
        </article>
        <article className="analysis-card pace-analysis">
          <div className="analysis-card-heading">
            <div>
              <small>{ar ? "حالة التقدم" : "PACE MIX"}</small>
              <h4>
                {ar
                  ? scope === "program"
                    ? "توزيع حالات تقدم الطلاب"
                    : "توزيع حالات طلابك"
                  : scope === "program"
                    ? "Program pace distribution"
                    : "Your students by pace"}
              </h4>
            </div>
            <span>
              {ar
                ? "الطلاب النشطون القابلون للقياس"
                : "Active measurable students"}
            </span>
          </div>
          <div className="pace-chart-body">
            <button
              type="button"
              className="pace-donut"
              aria-label={ar ? "مسح تحديد الحالة" : "Clear pace filter"}
              aria-pressed={selectedStatus === null}
              onClick={() => setSelectedStatus(null)}
              style={{ background: statusChart }}
            >
              <span>
                <strong>{paceRows.length}</strong>
                <small>{ar ? "طالب" : "students"}</small>
              </span>
            </button>
            <div className="pace-legend">
              {statusSlices.map((slice) => (
                <button
                  type="button"
                  aria-pressed={selectedStatus === slice.status}
                  className={`pace-legend-row${selectedStatus === slice.status ? " selected" : ""}`}
                  key={slice.status}
                  onClick={() =>
                    setSelectedStatus(
                      selectedStatus === slice.status ? null : slice.status,
                    )
                  }
                >
                  <i style={{ background: slice.color }} />
                  <span>{statusNames[lang][slice.status]}</span>
                  <b>{slice.value}</b>
                  <small>
                    {paceRows.length
                      ? Math.round((slice.value / paceRows.length) * 100)
                      : 0}
                    %
                  </small>
                </button>
              ))}
            </div>
          </div>
          {selectedStatus && (
            <div className="analysis-drilldown">
              {paceRows
                .filter((row) => row.paceStatus === selectedStatus)
                .map((student) => (
                  <button
                    type="button"
                    key={student.studentId}
                    onClick={() => onSelect(student)}
                  >
                    <span>
                      <b>{student.displayName || student.login}</b>
                      <small>@{student.login}</small>
                    </span>
                    <strong>{statusNames[lang][student.paceStatus]}</strong>
                  </button>
                ))}
            </div>
          )}
        </article>
        <article className="analysis-card month-analysis">
          <div className="analysis-card-heading">
            <div>
              <small>{ar ? "الموقع في المنهج" : "CURRICULUM POSITION"}</small>
              <h4>
                {ar ? "طلابك حسب شهر المنهج" : "Students by curriculum month"}
              </h4>
            </div>
            <span>
              {monthRows.length}{" "}
              {ar ? "مجموعات" : monthRows.length === 1 ? "group" : "groups"}
            </span>
          </div>
          <div className="month-chart">
            {monthRows.length ? (
              monthRows.map(([month, count]) => (
                <button
                  type="button"
                  aria-pressed={selectedMonth === month}
                  className={`month-chart-row${selectedMonth === month ? " selected" : ""}`}
                  key={month}
                  onClick={() =>
                    setSelectedMonth(selectedMonth === month ? null : month)
                  }
                >
                  <span>
                    {month > 0
                      ? ar
                        ? `الشهر ${month}`
                        : `Month ${month}`
                      : ar
                        ? "غير محدد"
                        : "Not recorded"}
                  </span>
                  <span>
                    <i style={{ width: `${(count / maxMonthCount) * 100}%` }} />
                  </span>
                  <b>{count}</b>
                </button>
              ))
            ) : (
              <p className="analysis-empty">
                {ar
                  ? "لا توجد بيانات منهج متاحة."
                  : "No curriculum data available yet."}
              </p>
            )}
          </div>
          {selectedMonth !== null && (
            <div className="analysis-drilldown">
              {rows
                .filter((row) => row.curriculumMonth === selectedMonth)
                .slice(0, 5)
                .map((student) => (
                  <button
                    type="button"
                    key={student.studentId}
                    onClick={() => onSelect(student)}
                  >
                    <span>
                      <b>{student.displayName || student.login}</b>
                      <small>@{student.login}</small>
                    </span>
                    <strong>{student.actualLevel}</strong>
                  </button>
                ))}
            </div>
          )}
        </article>
      </div>
      {includeProjects && (
        <ProjectProgress
          rows={comparisonRows ?? rows}
          accounts={accounts}
          mentors={mentors}
          lang={lang}
          view="cohort"
          onSelectStudent={onSelect}
        />
      )}
    </section>
  );
}

export function OverviewPanel({
  rows,
  accounts,
  lang,
  onSelect,
  labels,
}: {
  rows: StudentPaceRow[];
  accounts: PulseAccount[];
  lang: Language;
  onSelect: (s: StudentPaceRow) => void;
  labels: typeof text.en;
}) {
  const ar = lang === "ar";
  const needs = rows
    .filter(
      (student) =>
        student.paceStatus === "RED" || student.paceStatus === "YELLOW",
    )
    .sort(
      (a, b) =>
        (a.paceStatus === "RED" ? 0 : 1) - (b.paceStatus === "RED" ? 0 : 1),
    );
  const activeStatuses: PaceStatus[] = ["GREEN", "YELLOW", "RED"];
  const colors: Record<PaceStatus, string> = {
    GREEN: "#42b983",
    YELLOW: "#dfb24f",
    RED: "#df796b",
    PAUSED: "#87969b",
    INACTIVE: "#a9b0b4",
    NOT_MEASURABLE: "#82adbc",
  };
  const reasonNames: Record<string, string> = {
    BELOW_EXPECTED_LEVEL: ar
      ? "أقل من المستوى المتوقع"
      : "Below expected level",
    BELOW_MINIMUM_LEVEL: ar ? "أقل من الحد الأدنى" : "Below minimum level",
    CURRENTLY_PAUSED: ar ? "متوقف مؤقتًا" : "Currently paused",
    NO_TIMELINE: ar ? "لا يوجد جدول زمني" : "No curriculum timeline",
    LEFT_SCHOOL: ar ? "غادر الأكاديمية" : "Left the academy",
  };
  return (
    <div className="overview-grid">
      <section className="table-panel overview-card">
        <div className="table-heading">
          <div>
            <h2>{labels.distribution}</h2>
            <p>
              {ar
                ? "الطلاب النشطون ذوو بيانات منهج قابلة للقياس فقط"
                : "Active, measurable student records only"}
            </p>
          </div>
        </div>
        <div className="distribution-list">
          {activeStatuses.map((key) => {
            const value = rows.filter(
              (student) => student.paceStatus === key,
            ).length;
            const pct = rows.length
              ? Math.round((value / rows.length) * 100)
              : 0;
            return (
              <div className="distribution-row" key={key}>
                <span className={`status-badge ${key.toLowerCase()}`}>
                  <i />
                  {statusNames[lang][key]}
                </span>
                <div className="distribution-track">
                  <span style={{ width: `${pct}%`, background: colors[key] }} />
                </div>
                <b>{value}</b>
              </div>
            );
          })}
        </div>
      </section>
      <section className="table-panel overview-card">
        <div className="table-heading">
          <div>
            <h2>{labels.recent}</h2>
            <p>
              {ar
                ? "القائمة كاملة؛ افتح الطالب لمراجعة التفاصيل"
                : "Complete list · open a student to review details"}
            </p>
          </div>
        </div>
        {needs.length ? (
          needs.map((student) => (
            <button
              className="attention-row"
              key={student.studentId}
              onClick={() => onSelect(student)}
            >
              <StudentAvatar student={student} accounts={accounts} />
              <span className="attention-student-copy">
                <b>{student.displayName || student.login}</b>
                <small>@{student.login}</small>
                <small className="attention-reason">
                  {student.healthReasonCodes
                    .map(
                      (code) =>
                        reasonNames[code] ??
                        code.replaceAll("_", " ").toLowerCase(),
                    )
                    .join(" · ") ||
                    (ar ? "لا يوجد سبب مسجل" : "No reason code supplied")}
                </small>
              </span>
              <span
                className={`status-badge ${student.paceStatus.toLowerCase()}`}
              >
                <i />
                {statusNames[lang][student.paceStatus]}
              </span>
              <span className="attention-arrow">{ar ? "←" : "→"}</span>
            </button>
          ))
        ) : (
          <div className="empty-state">
            <b>
              {ar
                ? "لا يوجد طلاب يحتاجون متابعة"
                : "No students need a check-in right now."}
            </b>
          </div>
        )}
      </section>
    </div>
  );
}
