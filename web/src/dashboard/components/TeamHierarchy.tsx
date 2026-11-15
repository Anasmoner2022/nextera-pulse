import type { PulseAccount, PulseRole } from '../../identity';
import { ProfileAvatar } from '../../identity';
import type { StudentPaceRow } from '../../domain';
import type { Language } from '../config';

type HierarchyProps = {
  manager: PulseAccount;
  managers?: PulseAccount[];
  mentors: PulseAccount[];
  rows: StudentPaceRow[];
  accounts: PulseAccount[];
  viewer: PulseAccount;
  lang: Language;
  onOpenAccount: (account: PulseAccount) => void;
  onOpenStudent: (student: StudentPaceRow, account: PulseAccount) => void;
};

// Convert role identifiers to the labels used by the hierarchy cards.
const roleLabel = (role: PulseRole, ar: boolean) => role === 'manager' ? (ar ? 'مدير البرنامج' : 'Program Manager') : role === 'mentor' ? (ar ? 'منتور' : 'Mentor') : (ar ? 'طالب' : 'Student');

// Display the manager, mentors, and students as one navigable organization map.
export function TeamHierarchy({ manager, managers, mentors, rows, accounts, viewer, lang, onOpenAccount, onOpenStudent }: HierarchyProps) {
  const ar = lang === 'ar';
  // The hierarchy is an explicitly shared directory: every role can browse
  // all mentor and student profiles. Profile navigation remains read-only.
  const scopedRows = rows;
  const shownMentors = mentors;
  const shownManagers = managers && managers.length > 0 ? managers : [manager];

  // Resolve each student from the account link first, then fall back to the mentor roster.
  function assignedMentor(row: StudentPaceRow, mentor: PulseAccount, allAccounts: PulseAccount[]) {
    const studentAccount = allAccounts.find(account => account.role === 'student' && account.studentLogin?.trim().replace(/^@/, '').toLowerCase() === row.login.trim().replace(/^@/, '').toLowerCase());
    if (studentAccount?.mentorId) return studentAccount.mentorId === mentor.id;
    return (mentor.assignedStudentLogins ?? []).some(login => login.trim().replace(/^@/, '').toLowerCase() === row.login.trim().replace(/^@/, '').toLowerCase());
  }

  const studentsFor = (mentor: PulseAccount) => scopedRows.filter(row => assignedMentor(row, mentor, accounts));
  const unassigned = scopedRows.filter(row => !shownMentors.some(mentor => assignedMentor(row, mentor, accounts)));
  const openStudent = (student: StudentPaceRow) => {
    const account = accounts.find(item => item.role === 'student' && item.studentLogin?.trim().replace(/^@/, '').toLowerCase() === student.login.trim().replace(/^@/, '').toLowerCase())
      ?? { id: `student-record-${student.studentId}`, name: student.displayName || student.login, email: student.email || '', role: 'student' as const, permissions: [], studentLogin: student.login };
    onOpenStudent(student, account);
  };

  const scopeDescription = ar
    ? 'كل حسابات المنتورز والطلاب ظاهرة للجميع. اضغط على أي حساب لفتح ملفه الشخصي.'
    : 'All mentor and student accounts are visible to everyone. Select any account to open its profile.';

  return <section className="hierarchy-page" aria-label={ar ? 'الهيكل التنظيمي للفريق' : 'Team hierarchy'}>
    <header className="hierarchy-heading"><div><span className="eyebrow">{ar ? 'فريق البرنامج' : 'PROGRAM TEAM'}</span><h2>{ar ? 'الهيكل التنظيمي' : 'Team hierarchy'}</h2><p>{scopeDescription}</p></div><span className="hierarchy-count"><b>{shownMentors.length}</b> {ar ? 'منتور' : 'mentors'}<i /> <b>{scopedRows.length}</b> {ar ? 'طالب' : 'students'}</span></header>

    <div className="hierarchy-tree">
      <div className="hierarchy-root-line" aria-hidden="true" />
      {shownManagers.map(person => <button type="button" className={`hierarchy-person hierarchy-manager${person.id === viewer.id ? ' is-current' : ''}`} key={person.id} aria-current={person.id === viewer.id ? 'true' : undefined} onClick={() => onOpenAccount(person)}>
        <ProfileAvatar account={person} size="medium" />
        <span className="hierarchy-person-copy"><small>{roleLabel('manager', ar)}{person.id === viewer.id && <em className="hierarchy-current-badge">{ar ? 'أنت' : 'YOU'}</em>}</small><b>{person.name}</b><i>{person.email}</i></span>
        <span className="hierarchy-open-mark" aria-hidden="true">↗</span>
      </button>)}

      {shownMentors.length > 0 ? <div className="hierarchy-branches">
        {shownMentors.map(mentor => {
          const assigned = studentsFor(mentor);
          const isCurrentMentor = mentor.id === viewer.id;
          return <article className="hierarchy-branch" key={mentor.id}>
            <button type="button" className={`hierarchy-person hierarchy-mentor${isCurrentMentor ? ' is-current' : ''}`} aria-current={isCurrentMentor ? 'true' : undefined} onClick={() => onOpenAccount(mentor)}>
              <ProfileAvatar account={mentor} size="medium" />
              <span className="hierarchy-person-copy"><small>{roleLabel('mentor', ar)}{isCurrentMentor && <em className="hierarchy-current-badge">{ar ? 'أنت' : 'YOU'}</em>}</small><b>{mentor.name}</b><i>{mentor.email}</i></span>
              <span className="hierarchy-team-size">{assigned.length}<small>{ar ? 'طلاب' : assigned.length === 1 ? 'student' : 'students'}</small></span>
              <span className="hierarchy-open-mark" aria-hidden="true">↗</span>
            </button>
            {assigned.length ? <div className="hierarchy-students">{assigned.map(student => {
              const linkedAccount = accounts.find(account => account.role === 'student' && account.studentLogin?.trim().replace(/^@/, '').toLowerCase() === student.login.trim().replace(/^@/, '').toLowerCase());
              const isCurrentStudent = viewer.role === 'student' && (linkedAccount?.id === viewer.id || (viewer.studentLogin ?? viewer.email.split('@')[0]).trim().replace(/^@/, '').toLowerCase() === student.login.trim().replace(/^@/, '').toLowerCase());
              const studentAvatarAccount = linkedAccount ?? { name: student.displayName || student.login };
              return <button type="button" className={`hierarchy-person hierarchy-student${isCurrentStudent ? ' is-current' : ''}`} aria-current={isCurrentStudent ? 'true' : undefined} key={student.studentId} onClick={() => openStudent(student)}>
                <ProfileAvatar account={studentAvatarAccount} size="small" />
                <span className="hierarchy-person-copy"><small>{roleLabel('student', ar)} · {ar ? 'المستوى' : 'Level'} {student.actualLevel}{isCurrentStudent && <em className="hierarchy-current-badge">{ar ? 'أنت' : 'YOU'}</em>}</small><b>{student.displayName || student.login}</b><i>@{student.login}</i></span>
                <span className="hierarchy-open-mark" aria-hidden="true">↗</span>
              </button>;
            })}</div> : <p className="hierarchy-empty-branch">{ar ? 'لا يوجد طلاب ظاهرون لهذا الحساب حسب صلاحياتك.' : 'No students are visible for this account with your access.'}</p>}
          </article>;
        })}
      </div> : <p className="hierarchy-empty">{ar ? 'لا يوجد منتورز لعرضهم بعد.' : 'No mentors to display yet.'}</p>}

      {unassigned.length > 0 && <section className="hierarchy-unassigned"><header><span>{ar ? 'طلاب بدون منتور مسند' : 'Unassigned students'}</span><b>{unassigned.length}</b></header><div className="hierarchy-students">{unassigned.map(student => { const isCurrentStudent = viewer.role === 'student' && (viewer.studentLogin ?? viewer.email.split('@')[0]).trim().replace(/^@/, '').toLowerCase() === student.login.trim().replace(/^@/, '').toLowerCase(); return <button type="button" className={`hierarchy-person hierarchy-student${isCurrentStudent ? ' is-current' : ''}`} aria-current={isCurrentStudent ? 'true' : undefined} key={student.studentId} onClick={() => openStudent(student)}><ProfileAvatar account={{ name: student.displayName || student.login }} size="small" /><span className="hierarchy-person-copy"><small>{roleLabel('student', ar)}{isCurrentStudent && <em className="hierarchy-current-badge">{ar ? 'أنت' : 'YOU'}</em>}</small><b>{student.displayName || student.login}</b><i>@{student.login}</i></span><span className="hierarchy-open-mark" aria-hidden="true">↗</span></button>;})}</div></section>}
    </div>
  </section>;
}
