import { useState } from "react";
import type { FormEvent } from "react";
import type { PaceStatus, StudentPaceRow } from "../../domain";
import { ProfileAvatar } from "../../identity";
import type { PulseAccount, PulseSession } from "../../identity";
import {
  findAssignedMentor,
  findLinkedStudentAccount,
} from "../../studentAccountLink.js";
import { dateText, initials, statusNames, statuses } from "../config";
import type { Language } from "../config";
import type { Intervention } from "../config";
import { ProjectProgress } from "./project-progress";
import { OverviewSectionTabs } from "./common";
import { CohortLevelHistogram } from "./CohortLevelHistogram";

// Prefer the linked account photo and fall back to initials for unlinked records.
export function StudentAvatar({
  student,
  accounts,
}: {
  student: StudentPaceRow;
  accounts: PulseAccount[];
}) {
  const account = findLinkedStudentAccount(student, accounts);
  return account ? (
    <ProfileAvatar account={account} size="small" />
  ) : (
    <span className={`avatar student-avatar avatar-${student.studentId % 4}`}>
      {initials(student)}
    </span>
  );
}

// Show contact details from the account when available and synthetic values in demo mode.
export function StudentContactDetails({
  student,
  accounts,
  lang,
}: {
  student: StudentPaceRow;
  accounts: PulseAccount[];
  lang: Language;
}) {
  const ar = lang === "ar";
  const linkedAccount = findLinkedStudentAccount(student, accounts);
  const email =
    linkedAccount?.email || student.email || `${student.login}@example.com`;
  const age = student.age ?? 17 + (student.studentId % 6);
  const phone =
    student.phone ||
    `+20 10 ${String(student.studentId).slice(-4)} ${String(student.studentId * 31).slice(-4)}`;
  return (
    <section
      className="student-contact-card"
      aria-label={ar ? "بيانات التواصل" : "Student contact details"}
    >
      <span className="eyebrow">
        {ar ? "بيانات الطالب" : "STUDENT DETAILS"}
      </span>
      <div className="student-contact-grid">
        <div className="student-contact-item">
          <small>{ar ? "البريد الإلكتروني" : "Email address"}</small>
          <a href={`mailto:${email}`}>{email}</a>
        </div>
        <div className="student-contact-item">
          <small>{ar ? "العمر" : "Age"}</small>
          <b>
            {age} {ar ? "سنة" : "years"}
          </b>
        </div>
        <div className="student-contact-item">
          <small>{ar ? "رقم التليفون" : "Phone number"}</small>
          <a href={`tel:${phone.replace(/[^+\d]/g, "")}`}>{phone}</a>
        </div>
      </div>
    </section>
  );
}

// Make every roster row open the same detail view through mouse or keyboard input.
export function StudentRow({
  s,
  accounts,
  lang,
  onSelect,
}: {
  s: StudentPaceRow;
  accounts: PulseAccount[];
  lang: Language;
  onSelect: (student: StudentPaceRow) => void;
}) {
  const names = statusNames[lang];
  const gap =
    s.gapToMinimum === null
      ? "—"
      : `${s.gapToMinimum > 0 ? "+" : ""}${s.gapToMinimum}`;
  return (
    <tr
      className="student-row"
      tabIndex={0}
      onClick={() => onSelect(s)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect(s);
        }
      }}
    >
      <td>
        <div className="student-cell">
          <StudentAvatar student={s} accounts={accounts} />
          <span>
            <b>{s.displayName || s.login}</b>
            <small>@{s.login}</small>
          </span>
        </div>
      </td>
      <td>{dateText(s.enrolledAt, lang)}</td>
      <td>
        <span className="days-cell">
          {s.activeAcademicDays}
          <small>{lang === "ar" ? "يوم" : "days"}</small>
        </span>
      </td>
      <td>
        {s.curriculumMonth > 0
          ? lang === "ar"
            ? `الشهر ${s.curriculumMonth}`
            : `Month ${s.curriculumMonth}`
          : "—"}
      </td>
      <td>
        <span className="level-cell">
          {s.actualLevel}
          <small>{lang === "ar" ? "مستوى" : "level"}</small>
        </span>
      </td>
      <td>
        {s.minimumLevel ?? "—"} <span className="muted">/</span>{" "}
        {s.expectedLevel ?? "—"}
      </td>
      <td>
        <span
          className={`gap-value ${s.gapToMinimum !== null && s.gapToMinimum < 0 ? "negative" : s.gapToMinimum !== null && s.gapToMinimum > 0 ? "positive" : ""}`}
        >
          {gap}
        </span>
      </td>
      <td>
        <span className={`status-badge ${s.paceStatus.toLowerCase()}`}>
          <i />
          {names[s.paceStatus]}
        </span>
      </td>
      <td>
        <span className="source-status-chip">{s.sourceStatus || "—"}</span>
      </td>
      <td>
        <button
          className="row-more"
          aria-label={
            lang === "ar" ? "عرض تفاصيل الطالب" : "View student details"
          }
          onClick={(event) => {
            event.stopPropagation();
            onSelect(s);
          }}
        >
          ···
        </button>
      </td>
    </tr>
  );
}

// Keep the roster detail dialog focused on student data, history, and the next action.
export function StudentDetails({
  student: s,
  accounts,
  lang,
  onClose,
  onTakeAction,
  onViewProfile,
  canTakeAction,
  interventions = [],
}: {
  student: StudentPaceRow;
  accounts: PulseAccount[];
  lang: Language;
  onClose: () => void;
  onTakeAction: (student: StudentPaceRow) => void;
  onViewProfile: (student: StudentPaceRow) => void;
  canTakeAction: boolean;
  interventions?: Intervention[];
}) {
  const ar = lang === "ar";
  const reasons: Record<string, string> = ar
    ? {
        BELOW_EXPECTED_LEVEL: "أقل من المستوى المتوقع",
        BELOW_MINIMUM_LEVEL: "أقل من الحد الأدنى",
        CURRENTLY_PAUSED: "متوقف حاليًا",
        NO_TIMELINE: "لا يوجد جدول زمني",
        LEFT_SCHOOL: "غادر الأكاديمية",
      }
    : {
        BELOW_EXPECTED_LEVEL: "Below expected level",
        BELOW_MINIMUM_LEVEL: "Below minimum level",
        CURRENTLY_PAUSED: "Currently paused",
        NO_TIMELINE: "No curriculum timeline",
        LEFT_SCHOOL: "Left the academy",
      };
  const fields = [
    [ar ? "تاريخ الانضمام" : "Enrollment date", dateText(s.enrolledAt, lang)],
    [
      ar ? "حالة 01Edu (تجريبية)" : "01Edu source status (demo)",
      s.sourceStatus,
    ],
    [
      ar ? "الأيام الأكاديمية النشطة" : "Active academic days",
      String(s.activeAcademicDays),
    ],
    [
      ar ? "شهر المنهج" : "Curriculum month",
      s.curriculumMonth ? String(s.curriculumMonth) : "—",
    ],
    [ar ? "المستوى الحالي" : "Current level", String(s.actualLevel)],
    [
      ar ? "الحد الأدنى الرسمي" : "Official minimum",
      s.minimumLevel === null ? "—" : String(s.minimumLevel),
    ],
    [
      ar ? "المستوى المتوقع" : "Expected level",
      s.expectedLevel === null ? "—" : String(s.expectedLevel),
    ],
    [
      ar ? "هدف نقطة التحقق" : "Checkpoint target",
      s.checkpointTarget === null ? "—" : String(s.checkpointTarget),
    ],
    [
      ar ? "الفارق عن المتوقع" : "Gap to expected",
      s.gapToExpected === null
        ? "—"
        : `${s.gapToExpected > 0 ? "+" : ""}${s.gapToExpected}`,
    ],
  ];
  const studentInterventions = interventions
    .filter((item) => item.studentId === s.studentId)
    .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
  const roleName = (role?: string) =>
    role === "manager"
      ? ar
        ? "مدير البرنامج"
        : "Program Manager"
      : role === "mentor"
        ? ar
          ? "منتور"
          : "Mentor"
        : ar
          ? "عضو الفريق"
          : "Team member";
  const timestamp = (value: string) => {
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime())
      ? value
      : new Intl.DateTimeFormat(ar ? "ar-EG" : "en-GB", {
          dateStyle: "medium",
          timeStyle: "short",
        }).format(parsed);
  };
  return (
    <div
      className="detail-backdrop student-detail-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        className="detail-panel student-detail-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="student-detail-title"
      >
        <header className="detail-header">
          <div>
            <StudentAvatar student={s} accounts={accounts} />
            <div>
              <button
                className="student-detail-name-link"
                id="student-detail-title"
                onClick={() => onViewProfile(s)}
                title={ar ? "فتح حساب الطالب" : "Open student account"}
              >
                {s.displayName || s.login}
                <span aria-hidden="true">{ar ? "←" : "→"}</span>
              </button>
              <small>@{s.login}</small>
            </div>
          </div>
          <button
            className="detail-close"
            onClick={onClose}
            aria-label={ar ? "إغلاق" : "Close"}
          >
            ×
          </button>
        </header>
        <div className="detail-body">
          <div className="student-detail-status">
            <span className={`status-badge ${s.paceStatus.toLowerCase()}`}>
              <i />
              {statusNames[lang][s.paceStatus]}
            </span>
            <p className="detail-cohort">
              {ar ? "المجموعة" : "Cohort"} · 01 Coding - Second Cohort
            </p>
          </div>
          <StudentContactDetails student={s} accounts={accounts} lang={lang} />
          <div className="detail-grid">
            {fields.map(([label, value]) => (
              <div className="detail-field" key={label}>
                <small>{label}</small>
                <b>{value}</b>
              </div>
            ))}
          </div>
          <div className="reason-box">
            <b>{ar ? "ملاحظات الحالة" : "Status notes"}</b>
            {s.healthReasonCodes.length ? (
              s.healthReasonCodes.map((code) => (
                <span key={code}>{reasons[code] ?? code}</span>
              ))
            ) : (
              <span>
                {ar ? "لا توجد ملاحظات إضافية" : "No additional notes"}
              </span>
            )}
          </div>
          <section
            className="student-intervention-history"
            aria-labelledby="student-intervention-title"
          >
            <header>
              <div>
                <span className="eyebrow">
                  {ar ? "سجل المتابعة" : "FOLLOW-UP HISTORY"}
                </span>
                <h3 id="student-intervention-title">
                  {ar ? "التعليقات والتدخلات" : "Comments and interventions"}
                </h3>
              </div>
              <span className="student-intervention-count">
                {studentInterventions.length}
              </span>
            </header>
            {studentInterventions.length ? (
              <div className="student-intervention-list">
                {studentInterventions.map((item) => (
                  <article className="student-intervention-note" key={item.id}>
                    <span className="student-intervention-marker">↗</span>
                    <div className="student-intervention-copy">
                      <p>{item.note}</p>
                      <small>
                        <b>
                          {item.authorName ||
                            (ar ? "عضو الفريق" : "Team member")}
                        </b>
                        <span
                          className={
                            item.authorName?.trim().toLowerCase() ===
                            roleName(item.authorRole).toLowerCase()
                              ? "student-intervention-role-redundant"
                              : undefined
                          }
                        >
                          {roleName(item.authorRole)}
                        </span>
                        <time dateTime={item.createdAt}>
                          {timestamp(item.createdAt)}
                        </time>
                        {item.performedByName && (
                          <span className="intervention-operator">
                            {ar
                              ? `نفّذها ${item.performedByName}`
                              : `Performed by ${item.performedByName}`}
                          </span>
                        )}
                      </small>
                    </div>
                  </article>
                ))}
              </div>
            ) : (
              <p className="student-intervention-empty">
                {ar
                  ? "لا توجد تعليقات أو تدخلات مسجلة لهذا الطالب حتى الآن."
                  : "No comments or interventions have been recorded for this student yet."}
              </p>
            )}
          </section>
          <div className="student-detail-footer">
            <p className="detail-footnote">
              {ar
                ? "حالة التقدم والمقاييس محسوبة من مصدر البيانات التجريبي."
                : "Pace status and metrics are provided by the demo data source."}
            </p>
            <div className="student-detail-actions">
              {canTakeAction && (
                <button
                  className="primary-button student-action-button"
                  onClick={() => onTakeAction(s)}
                >
                  {ar ? "اتخذ إجراء مع الطالب" : "Take action with student"}{" "}
                  <span>{ar ? "←" : "→"}</span>
                </button>
              )}
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

// Expose basic student editing while account creation stays in Accounts & access.
export function ManageStudents({
  rows,
  accounts,
  lang,
  onEdit,
  onDelete,
  readOnly = false,
}: {
  rows: StudentPaceRow[];
  accounts: PulseAccount[];
  lang: Language;
  onEdit: (student: StudentPaceRow) => void;
  onDelete: (student: StudentPaceRow) => void;
  readOnly?: boolean;
}) {
  const ar = lang === "ar";
  return (
    <section className="table-panel manage-panel">
      <div className="table-heading">
        <div>
          <span className="eyebrow">
            {ar ? "مساحة المنتور" : "MENTOR WORKSPACE"}
          </span>
          <h2>{ar ? "إدارة الطلاب" : "Manage students"}</h2>
          <p>
            {ar
              ? "الحسابات الجديدة تُنشأ من صفحة الحسابات والصلاحيات. المقاييس الرسمية تأتي من مصدر الأكاديمية."
              : "Create student accounts in Accounts & access. Official learning metrics come from the academy source."}
          </p>
        </div>
      </div>
      {readOnly && (
        <p className="api-readonly-note" role="status">
          Read-only API connection: edit and remove require backend write
          endpoints.
        </p>
      )}
      <div className="manage-list">
        {rows.map((student) => (
          <article
            className="manage-row manage-student-metrics"
            key={student.studentId}
          >
            <div className="manage-student-heading">
              <StudentAvatar student={student} accounts={accounts} />
              <div className="manage-identity">
                <b>{student.displayName || student.login}</b>
                <small>@{student.login}</small>
              </div>
              <span
                className={`status-badge ${student.paceStatus.toLowerCase()}`}
              >
                <i />
                {statusNames[lang][student.paceStatus]}
              </span>
            </div>
            <div className="manage-source-metrics">
              <span>
                <small>{ar ? "الشهر" : "Month"}</small>
                <b>
                  {student.curriculumMonth || "—"}
                  {student.curriculumRound
                    ? ` · Round ${student.curriculumRound}`
                    : ""}
                </b>
              </span>
              <span>
                <small>{ar ? "المستوى" : "Level"}</small>
                <b>{student.actualLevel}</b>
              </span>
              <span>
                <small>
                  {ar ? "الحد الأدنى / المتوقع" : "Minimum / expected"}
                </small>
                <b>
                  {student.minimumLevel ?? "—"} / {student.expectedLevel ?? "—"}
                </b>
              </span>
              <span>
                <small>{ar ? "الفارق عن المتوقع" : "Gap to expected"}</small>
                <b>
                  {student.gapToExpected === null
                    ? "—"
                    : `${student.gapToExpected > 0 ? "+" : ""}${student.gapToExpected}`}
                </b>
              </span>
              <span>
                <small>01Edu status</small>
                <b className="source-status-chip">
                  {student.sourceStatus || "—"}
                </b>
              </span>
            </div>
            {!readOnly && (
              <div className="manage-actions">
                <button
                  className="setting-action"
                  onClick={() => onEdit(student)}
                >
                  {ar ? "تعديل" : "Edit"}
                </button>
                <button
                  className="setting-action danger-action"
                  onClick={() => onDelete(student)}
                >
                  {ar ? "حذف" : "Remove"}
                </button>
              </div>
            )}
          </article>
        ))}
      </div>
      {!readOnly && (
        <p className="settings-note">
          {ar
            ? "تغييرات النسخة التجريبية محفوظة في هذا المتصفح. حالة التقدم الرسمية للـAPI."
            : "Demo edits are stored in this browser. Official pace status remains owned by the API."}
        </p>
      )}
    </section>
  );
}

// Edit the demo identity and pace fields without changing the source API record.
export function StudentForm({
  student,
  lang,
  onSave,
  onClose,
}: {
  student: StudentPaceRow;
  lang: Language;
  onSave: (student: StudentPaceRow) => void;
  onClose: () => void;
}) {
  const ar = lang === "ar";
  const [name, setName] = useState(student.displayName ?? "");
  const [login, setLogin] = useState(student.login);
  const [level, setLevel] = useState(student.actualLevel);
  const [status, setStatus] = useState<PaceStatus>(student.paceStatus);
  const [enrolledAt, setEnrolledAt] = useState(student.enrolledAt);
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSave({
      ...student,
      displayName: name.trim(),
      login: login.trim(),
      actualLevel: Number(level),
      paceStatus: status,
      enrolledAt,
    });
  };
  return (
    <div
      className="detail-backdrop student-form-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        className="student-form-card"
        role="dialog"
        aria-modal="true"
        aria-labelledby="student-edit-title"
      >
        <button
          className="detail-close"
          onClick={onClose}
          aria-label={ar ? "إغلاق" : "Close"}
        >
          ×
        </button>
        <span className="eyebrow">
          {ar ? "إدارة المجموعة" : "COHORT MANAGEMENT"}
        </span>
        <h2 id="student-edit-title">
          {ar ? "تعديل بيانات الطالب" : "Edit student"}
        </h2>
        <p>
          {ar
            ? "حدّث بيانات الطالب الأساسية."
            : "Update the student’s basic information."}
        </p>
        <form className="admin-form student-edit-form" onSubmit={submit}>
          <label>
            {ar ? "اسم الطالب" : "Student name"}
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
            />
          </label>
          <label>
            {ar ? "اسم الدخول" : "Login"}
            <input
              value={login}
              onChange={(e) => setLogin(e.target.value)}
              placeholder="student.name"
              required
            />
          </label>
          <div className="form-two">
            <label>
              {ar ? "المستوى الحالي (تجريبي)" : "Current level (demo)"}
              <input
                type="number"
                min="0"
                value={level}
                onChange={(e) => setLevel(Number(e.target.value))}
                required
              />
            </label>
            <label>
              {ar ? "حالة تجريبية" : "Demo status"}
              <select
                value={status}
                onChange={(e) => setStatus(e.target.value as PaceStatus)}
              >
                {statuses.map((value) => (
                  <option key={value} value={value}>
                    {statusNames[lang][value]}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <label>
            {ar ? "تاريخ الانضمام" : "Enrollment date"}
            <input
              type="date"
              value={enrolledAt}
              onChange={(e) => setEnrolledAt(e.target.value)}
              required
            />
          </label>
          <div className="form-warning">
            {ar
              ? "المستوى وحالة التقدم تجريبيان حاليًا؛ البيانات الرسمية ستأتي من الـAPI."
              : "Level and pace status are demo values for now. Official data will come from the API."}
          </div>
          <div className="form-actions">
            <button className="setting-action" type="button" onClick={onClose}>
              {ar ? "إلغاء" : "Cancel"}
            </button>
            <button className="primary-button" type="submit">
              {ar ? "حفظ التعديلات" : "Save changes"}
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}

// Compare student levels without overcrowding the chart with student names.
export function StudentCohortComparison({
  student,
  cohortRows,
  accounts,
  mentors,
  lang,
}: {
  student: StudentPaceRow;
  cohortRows: StudentPaceRow[];
  accounts: PulseAccount[];
  mentors: PulseAccount[];
  lang: Language;
  isOwnProfile?: boolean;
}) {
  const ar = lang === "ar";
  const sorted = cohortRows
    .slice()
    .sort(
      (a, b) =>
        a.actualLevel - b.actualLevel ||
        (a.displayName || a.login).localeCompare(b.displayName || b.login),
    );
  const rank =
    Math.max(
      0,
      sorted.findIndex((row) => row.studentId === student.studentId),
    ) + 1;
  const median = sorted.length
    ? sorted[Math.floor((sorted.length - 1) / 2)].actualLevel
    : 0;
  return (
    <section className="table-panel student-cohort-summary">
      <CohortLevelHistogram
        rows={cohortRows}
        accounts={accounts}
        mentors={mentors}
        lang={lang}
        highlightedStudentId={student.studentId}
      />
      <div className="student-peer-summary">
        <div>
          <small>{ar ? "ترتيبك حسب المستوى" : "Your position by level"}</small>
          <b>
            {rank} <span>/ {sorted.length}</span>
          </b>
        </div>
        <div>
          <small>{ar ? "مستواك الحالي" : "Your current level"}</small>
          <b>{student.actualLevel}</b>
        </div>
        <div>
          <small>{ar ? "وسيط المجموعة" : "Cohort median"}</small>
          <b>{median}</b>
        </div>
      </div>
    </section>
  );
}
// Keep student navigation in the overview while rendering only the selected section.
export function StudentHome({
  student,
  lang,
  cohortRows,
  interventions,
  accounts,
  mentors,
  viewer,
  onOpenProfile,
  activeTab,
  onActiveTabChange,
}: {
  student: StudentPaceRow | null;
  lang: Language;
  cohortRows: StudentPaceRow[];
  interventions: Intervention[];
  accounts: PulseAccount[];
  mentors: PulseAccount[];
  viewer: PulseSession;
  onOpenProfile: (account: PulseAccount) => void;
  activeTab: string;
  onActiveTabChange: (id: string) => void;
}) {
  const ar = lang === "ar";
  if (!student)
    return (
      <section className="table-panel student-home">
        <h2>
          {ar ? "لم يتم العثور على بيانات الطالب" : "Student profile not found"}
        </h2>
        <p>
          {ar
            ? "اطلب من المنتور ربط حسابك ببيانات الأكاديمية."
            : "Ask your mentor to link your account to an academy record."}
        </p>
      </section>
    );

  const studentInterventions = interventions
    .filter((item) => item.studentId === student.studentId)
    .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
  const assignedMentor =
    findAssignedMentor(student, accounts, mentors) ??
    mentors.find((mentor) => mentor.id === viewer.mentorId);
  const roleLabel = (role?: string) =>
    role === "manager"
      ? ar
        ? "مدير البرنامج"
        : "Program Manager"
      : role === "mentor"
        ? ar
          ? "منتور"
          : "Mentor"
        : ar
          ? "عضو الفريق"
          : "Team member";
  const timestamp = (value: string) => {
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime())
      ? value
      : new Intl.DateTimeFormat(ar ? "ar-EG" : "en-GB", {
          dateStyle: "medium",
          timeStyle: "short",
        }).format(parsed);
  };

  return (
    <>
      <div className="student-welcome">
        <span className="eyebrow">{ar ? "مساحة الطالب" : "STUDENT SPACE"}</span>
        <h2>
          {ar
            ? `أهلًا ${student.displayName || student.login}، رحلتك ماشية إزاي؟`
            : `Welcome, ${student.displayName || student.login}. Here is your learning pulse.`}
        </h2>
        <p>
          {ar
            ? "دي نظرة على تقدمك في المجموعة."
            : "A personal snapshot of your progress in the cohort."}
        </p>
      </div>
      <OverviewSectionTabs
        label={ar ? "أقسام مساحة الطالب" : "Student overview sections"}
        active={activeTab}
        onChange={onActiveTabChange}
        items={[
          { id: "progress", icon: "↗", label: ar ? "تقدمي" : "My progress" },
          {
            id: "cohort",
            icon: "⌁",
            label: ar ? "مكاني في الكوهورت" : "Cohort comparison",
          },
          { id: "projects", icon: "◫", label: ar ? "مشاريعي" : "My projects" },
          {
            id: "followups",
            icon: "▤",
            label: ar ? "المتابعات" : "Follow-ups",
          },
        ]}
      />
      <div className="student-tab-content" key={`student-tab-${activeTab}`}>
        {activeTab === "progress" && (
          <div className="student-progress-layout">
            <section className="table-panel student-home">
              <div className="student-progress-head">
                <div>
                  <span className="eyebrow">
                    {ar ? "حالة التقدم" : "YOUR PACE"}
                  </span>
                  <h2>{ar ? "مستواك الحالي" : "Your current level"}</h2>
                </div>
                <span
                  className={`status-badge ${student.paceStatus.toLowerCase()}`}
                >
                  <i />
                  {statusNames[lang][student.paceStatus]}
                </span>
              </div>
              <div className="student-progress">
                <div className="student-level">
                  {student.actualLevel}
                  <small>{ar ? "المستوى" : "LEVEL"}</small>
                </div>
                <div>
                  <b>
                    {student.minimumLevel === null
                      ? ar
                        ? "بيانات الخطة لم تتصل بعد"
                        : "Your learning plan is not connected yet"
                      : ar
                        ? "استمر في خطوتك القادمة"
                        : "Keep moving toward your next milestone"}
                  </b>
                  <p>
                    {ar
                      ? "سيظهر هنا الحد الأدنى والمستوى المتوقع بعد ربط بيانات المنهج."
                      : "Minimum and expected levels will appear here once curriculum data is connected."}
                  </p>
                </div>
              </div>
              <div className="student-metrics">
                <div>
                  <small>{ar ? "أيام نشطة" : "Active days"}</small>
                  <b>{student.activeAcademicDays}</b>
                </div>
                <div>
                  <small>{ar ? "شهر المنهج" : "Curriculum month"}</small>
                  <b>{student.curriculumMonth || "—"}</b>
                </div>
                <div>
                  <small>{ar ? "تاريخ الانضمام" : "Joined"}</small>
                  <b>{dateText(student.enrolledAt, lang)}</b>
                </div>
              </div>
            </section>
            <section className="table-panel student-mentor-card">
              <div>
                <span className="eyebrow">
                  {ar ? "المنتور المسؤول" : "YOUR INSTRUCTOR"}
                </span>
                <p>
                  {ar
                    ? "المنتور المسؤول عن متابعتك في البرنامج."
                    : "The mentor guiding your progress in the program."}
                </p>
              </div>
              {assignedMentor ? (
                <div className="student-mentor-identity">
                  <ProfileAvatar account={assignedMentor} size="medium" />
                  <span>
                    <b>{assignedMentor.name}</b>
                    <small>
                      {ar ? "منتور" : "Instructor"} · {assignedMentor.email}
                    </small>
                  </span>
                </div>
              ) : (
                <span className="student-mentor-unassigned">
                  {ar ? "لم يتم تعيين منتور بعد" : "No instructor assigned yet"}
                </span>
              )}
            </section>
          </div>
        )}
        {activeTab === "projects" && (
          <ProjectProgress
            rows={[student]}
            accounts={accounts}
            mentors={mentors}
            lang={lang}
            view="student"
          />
        )}
        {activeTab === "cohort" && (
          <StudentCohortComparison
            student={student}
            cohortRows={cohortRows}
            accounts={accounts}
            mentors={mentors}
            lang={lang}
          />
        )}
        {activeTab === "followups" && (
          <section className="table-panel student-home-followups">
            <header>
              <div>
                <span className="eyebrow">
                  {ar ? "متابعات فريقك" : "YOUR FOLLOW-UPS"}
                </span>
                <h2>{ar ? "رسائل من المنتورز" : "Notes from your mentors"}</h2>
              </div>
              <span className="student-intervention-count">
                {studentInterventions.length}
              </span>
            </header>
            {studentInterventions.length ? (
              <div className="student-home-followup-list">
                {studentInterventions.map((item) => {
                  const author = accounts.find(
                    (account) => account.id === item.authorId,
                  ) ??
                    (item.authorId === viewer.id ? viewer : undefined) ?? {
                      id: item.authorId || `followup-author-${item.id}`,
                      name:
                        item.authorName || (ar ? "عضو الفريق" : "Team member"),
                      email: "",
                      role:
                        item.authorRole === "manager"
                          ? ("manager" as const)
                          : ("mentor" as const),
                      permissions: [],
                      quote: "",
                    };
                  return (
                    <article
                      className="student-home-followup"
                      data-intervention-id={item.id}
                      key={item.id}
                    >
                      <header>
                        <button
                          className="student-home-author"
                          type="button"
                          onClick={() => onOpenProfile(author)}
                          aria-label={
                            ar
                              ? `عرض الملف الشخصي لـ ${author.name}`
                              : `View ${author.name}'s profile`
                          }
                        >
                          <ProfileAvatar account={author} size="small" />
                          <span>
                            <b>{author.name}</b>
                            <small>{roleLabel(item.authorRole)}</small>
                          </span>
                          <i aria-hidden="true">{ar ? "←" : "→"}</i>
                        </button>
                        <time dateTime={item.createdAt}>
                          {timestamp(item.createdAt)}
                        </time>
                      </header>
                      <p>{item.note}</p>
                      {item.performedByName && (
                        <small className="intervention-operator">
                          {ar
                            ? `نفّذها ${item.performedByName}`
                            : `Performed by ${item.performedByName}`}
                        </small>
                      )}
                    </article>
                  );
                })}
              </div>
            ) : (
              <p className="student-followups-empty">
                {ar ? "لا توجد متابعات جديدة حتى الآن." : "No follow-ups yet."}
              </p>
            )}
          </section>
        )}
      </div>
    </>
  );
}
