import { ProfileAvatar } from '../../identity';
import type { PulseAccount } from '../../identity';
import type { StudentPaceRow } from '../../domain';
import { normalizeLogin } from '../config';
import type { Intervention, Language } from '../config';
import { isInActiveKpiPopulation } from '../../domain';
import { MentorAnalytics } from './overview';
import { StudentAvatar, StudentRow } from './students';

const permissionLabels: Record<string, string> = {
  viewProgress: 'View learning progress',
  viewCohort: 'View cohort summary',
  manageStudents: 'Manage student records',
  manageAccounts: 'Create accounts and assign access',
  manageInterventions: 'Manage follow-ups',
  viewAllStudents: 'View all students',
  viewMentorPerformance: 'View mentor performance',
};

export function MentorWorkspaceView({ mentor, students, accounts, interventions, lang, onSelectStudent, onBack, onActAs }: {
  mentor: PulseAccount;
  students: StudentPaceRow[];
  accounts: PulseAccount[];
  interventions: Intervention[];
  lang: Language;
  onSelectStudent: (student: StudentPaceRow) => void;
  onBack: () => void;
  onActAs: () => void;
}) {
  const assignedLogins = new Set((mentor.assignedStudentLogins ?? []).map(normalizeLogin));
  const mentorStudents = students.filter(student => assignedLogins.has(normalizeLogin(student.login)) || accounts.some(account => account.role === 'student' && account.mentorId === mentor.id && normalizeLogin(account.studentLogin ?? '') === normalizeLogin(student.login)));
  const studentIds = new Set(mentorStudents.map(student => student.studentId));
  const mentorInterventions = interventions.filter(item => studentIds.has(item.studentId)).sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
  const active = mentorStudents.filter(isInActiveKpiPopulation);
  const onTrack = active.filter(student => student.paceStatus === 'GREEN').length;
  const attention = active.filter(student => student.paceStatus === 'YELLOW' || student.paceStatus === 'RED').length;
  const ar = lang === 'ar';
  const roleName = (role?: string) => role === 'manager' ? (ar ? 'مدير البرنامج' : 'Program Manager') : role === 'mentor' ? (ar ? 'منتور' : 'Mentor') : (ar ? 'عضو الفريق' : 'Team member');
  const date = (value: string) => new Intl.DateTimeFormat(ar ? 'ar-EG' : 'en-GB', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));

  return <div className="mentor-workspace-view">
    <section className="table-panel mentor-workspace-hero">
      <div className="mentor-workspace-topline"><button type="button" className="setting-action" onClick={onBack}>← {ar ? 'كل المنتورز' : 'All mentors'}</button><button type="button" className="primary-button" onClick={onActAs}>{ar ? `العمل كـ ${mentor.name}` : `Act as ${mentor.name}`}</button></div>
      <div className="mentor-workspace-identity"><ProfileAvatar account={mentor} size="large" /><div><span className="eyebrow">{ar ? 'حساب المنتور' : 'MENTOR ACCOUNT'}</span><h2>{mentor.name}</h2><p>{mentor.email}</p></div><span className="mentor-workspace-capacity">{mentorStudents.length}<small> / {mentor.capacityLimit ?? '—'} {ar ? 'طالب' : 'students'}</small></span></div>
      {mentor.quote?.trim() && <blockquote className="mentor-workspace-quote">“{mentor.quote}”</blockquote>}
      <div className="mentor-workspace-permissions"><b>{ar ? 'صلاحيات الحساب' : 'Account permissions'}</b><div>{mentor.permissions.map(permission => <span key={permission}>{permissionLabels[permission] ?? permission}</span>)}{mentor.permissions.length === 0 && <span>{ar ? 'لا توجد صلاحيات مسجلة' : 'No permissions recorded'}</span>}</div></div>
    </section>
    <div className="mentor-workspace-metrics"><article><small>{ar ? 'الطلاب المسندون' : 'Assigned students'}</small><strong>{mentorStudents.length}</strong></article><article><small>{ar ? 'على المسار' : 'On track'}</small><strong>{onTrack}</strong></article><article><small>{ar ? 'يحتاجون متابعة' : 'Need attention'}</small><strong>{attention}</strong></article><article><small>{ar ? 'المتابعات المسجلة' : 'Follow-ups'}</small><strong>{mentorInterventions.length}</strong></article></div>
    <MentorAnalytics rows={mentorStudents} interventions={mentorInterventions} lang={lang} onSelect={onSelectStudent} />
    <section className="table-panel mentor-workspace-roster"><div className="table-heading"><div><span className="eyebrow">{ar ? 'مجموعة المنتور' : 'MENTOR ROSTER'}</span><h2>{ar ? 'الطلاب المسندون' : 'Assigned students'}</h2><p>{ar ? 'افتح أي طالب لمراجعة تقدمه وسجل متابعته.' : 'Open a student to review their progress and follow-up history.'}</p></div><span className="cohort-pill">{mentorStudents.length} {ar ? 'طالب' : 'students'}</span></div><div className="table-scroll"><table><thead><tr><th>{ar ? 'الطالب' : 'Student'}</th><th>{ar ? 'أيام نشطة' : 'Active days'}</th><th>{ar ? 'شهر المنهج' : 'Curriculum month'}</th><th>{ar ? 'المستوى' : 'Level'}</th><th>{ar ? 'حالة التقدم' : 'Pace status'}</th><th /></tr></thead><tbody>{mentorStudents.map(student => <StudentRow key={student.studentId} s={student} accounts={accounts} lang={lang} onSelect={onSelectStudent} />)}{mentorStudents.length === 0 && <tr><td colSpan={6}><div className="empty-state"><b>{ar ? 'لا يوجد طلاب مسندون لهذا المنتور حاليًا.' : 'No students are assigned to this mentor yet.'}</b></div></td></tr>}</tbody></table></div></section>
    <section className="table-panel mentor-workspace-followups"><div className="table-heading"><div><span className="eyebrow">{ar ? 'سجل الفريق' : 'TEAM ACTIVITY'}</span><h2>{ar ? 'المتابعات على طلابه' : 'Follow-ups for their students'}</h2><p>{ar ? 'المتابعات مكتوبة بواسطة أي عضو فريق على الطلاب المسندين لهذا المنتور.' : 'Notes from any team member about this mentor’s assigned students.'}</p></div><span className="student-intervention-count">{mentorInterventions.length}</span></div>{mentorInterventions.length ? <div className="mentor-workspace-followup-list">{mentorInterventions.map(item => { const student = mentorStudents.find(row => row.studentId === item.studentId); return <article key={item.id}><StudentAvatar student={student!} accounts={accounts} /><div><button type="button" onClick={() => student && onSelectStudent(student)}>{student?.displayName || student?.login}</button><p>{item.note}</p><small><b>{item.authorName || (ar ? 'عضو الفريق' : 'Team member')}</b><span>{roleName(item.authorRole)}</span><time dateTime={item.createdAt}>{date(item.createdAt)}</time>{item.performedByName && <span className="intervention-operator">{ar ? `نفّذها ${item.performedByName}` : `Performed by ${item.performedByName}`}</span>}</small></div></article>; })}</div> : <p className="mentor-workspace-empty">{ar ? 'لا توجد متابعات مسجلة على طلاب هذا المنتور.' : 'No follow-ups have been recorded for this mentor’s students.'}</p>}</section>
  </div>;
}
