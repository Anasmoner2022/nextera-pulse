import type { StudentPaceRow } from '../../domain';
import { ProfileAvatar } from '../../identity';
import type { PulseAccount } from '../../identity';
import { findAssignedMentor } from '../../studentAccountLink.js';
import type { Intervention, Language } from '../config';
import { statusNames } from '../config';
import { ProjectProgress } from './project-progress';
import { StudentCohortComparison } from './students';

// Compose the read-only student profile with the same cohort and project insights used in the dashboard.
export function StudentProfileInsights({ student, cohortRows, accounts, mentors, interventions, lang }: { student: StudentPaceRow; cohortRows: StudentPaceRow[]; accounts: PulseAccount[]; mentors: PulseAccount[]; interventions: Intervention[]; lang: Language; }) {
  const ar = lang === 'ar';
  const mentor = findAssignedMentor(student, accounts, mentors);
  const studentInterventions = interventions.filter(item => item.studentId === student.studentId).sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
  const roleName = (role?: string) => role === 'manager' ? (ar ? 'مدير البرنامج' : 'Program Manager') : role === 'mentor' ? (ar ? 'منتور' : 'Mentor') : (ar ? 'عضو الفريق' : 'Team member');
  const timestamp = (value: string) => { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(ar ? 'ar-EG' : 'en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(date); };
  // Keep the headline measures aligned with the student and cohort dashboards.
  const metrics = [
    { label: ar ? 'حالة التقدم' : 'Pace status', value: statusNames[lang][student.paceStatus], status: student.paceStatus },
    { label: ar ? 'المستوى الحالي' : 'Current level', value: String(student.actualLevel) },
    { label: ar ? 'المستوى المتوقع' : 'Expected level', value: student.expectedLevel === null ? '—' : String(student.expectedLevel) },
    { label: ar ? 'الفارق عن المتوقع' : 'Gap to expected', value: student.gapToExpected === null ? '—' : `${student.gapToExpected > 0 ? '+' : ''}${student.gapToExpected}` },
    { label: ar ? 'أيام أكاديمية نشطة' : 'Active academic days', value: String(student.activeAcademicDays) },
  ];

  return <section className="student-profile-insights" aria-label={ar ? 'تحليلات الطالب' : 'Student insights'}>
    <header className="student-profile-insights-heading"><div><span className="eyebrow">{ar ? 'ملخص الطالب' : 'STUDENT SNAPSHOT'}</span><h2>{ar ? `تحليلات ${student.displayName || student.login}` : `${student.displayName || student.login} insights`}</h2><p>{ar ? 'التقدم والمقارنة والمشاريع وسجل المتابعة في مكان واحد.' : 'Progress, cohort position, projects, and follow-up history in one place.'}</p></div><span className="student-profile-demo-tag"><i />{ar ? 'بيانات تجريبية' : 'Demo data'}</span></header>

    <section className="table-panel student-profile-pace-card">
      <header><div><span className="eyebrow">{ar ? 'المسار الأكاديمي' : 'LEARNING PACE'}</span><h3>{ar ? 'الحالة والمستوى الحالي' : 'Status and current level'}</h3></div><span className={`status-badge ${student.paceStatus.toLowerCase()}`}><i />{statusNames[lang][student.paceStatus]}</span></header>
      <div className="student-profile-metrics">{metrics.map(metric => <article key={metric.label}><small>{metric.label}</small>{metric.status ? <span className={`status-badge ${metric.status.toLowerCase()}`}><i />{metric.value}</span> : <b>{metric.value}</b>}</article>)}</div>
      <div className="student-profile-mentor"><span className="student-profile-mentor-mark">♙</span><span><small>{ar ? 'المنتور المسؤول' : 'ASSIGNED MENTOR'}</small><b>{mentor?.name ?? (ar ? 'غير مسند' : 'Unassigned')}</b><i>{mentor?.email ?? (ar ? 'لم يتم تعيين منتور لهذا الطالب' : 'No mentor is assigned to this student yet')}</i></span></div>
    </section>

    <StudentCohortComparison key={student.studentId} student={student} cohortRows={cohortRows} accounts={accounts} mentors={mentors} lang={lang} isOwnProfile={false} />
    <ProjectProgress rows={[student]} accounts={accounts} mentors={mentors} lang={lang} view="student" />

    <section className="table-panel student-profile-interventions">
      <header><div><span className="eyebrow">{ar ? 'سجل المتابعة' : 'FOLLOW-UP HISTORY'}</span><h3>{ar ? 'التدخلات والملاحظات' : 'Interventions and notes'}</h3><p>{ar ? 'من كتب كل متابعة ودوره ووقت تسجيلها.' : 'Who wrote each follow-up, their role, and when it was recorded.'}</p></div><span className="student-intervention-count">{studentInterventions.length}</span></header>
      {studentInterventions.length ? <div className="student-profile-intervention-list">{studentInterventions.map(item => {
        const author = accounts.find(account => account.id === item.authorId)
          ?? accounts.find(account => item.authorName && account.name === item.authorName);
        const authorName = item.authorName || author?.name || (ar ? 'عضو الفريق' : 'Team member');
        const authorRole = item.authorRole || author?.role;
        return <article className="student-profile-intervention" key={item.id}>
          <header className="student-profile-intervention-author">
            {author
              ? <ProfileAvatar account={author} size="small" />
              : <span className="student-profile-intervention-avatar-fallback" aria-hidden="true">{authorName.split(/\s+/).slice(0, 2).map(part => part[0]).join('').toUpperCase()}</span>}
            <div><b>{authorName}</b><span className={`role-pill ${authorRole === 'manager' ? 'manager' : 'mentor'}`}>{roleName(authorRole)}</span></div>
            <time dateTime={item.createdAt}>{timestamp(item.createdAt)}</time>
          </header>
          <p>{item.note}</p>
          {item.performedByName && <small className="student-profile-intervention-operator">{ar ? `نفذها ${item.performedByName}` : `Performed by ${item.performedByName}`}</small>}
        </article>;
      })}</div> : <p className="student-profile-no-interventions">{ar ? 'لا توجد تدخلات أو ملاحظات مسجلة لهذا الطالب حتى الآن.' : 'No interventions or notes have been recorded for this student yet.'}</p>}
    </section>
  </section>;
}
