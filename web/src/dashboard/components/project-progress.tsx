import { useEffect, useState } from 'react';
import type { StudentPaceRow } from '../../domain';
import { getCohortProgress, getStudentProgress, usesLiveApi, type ProgressBoard } from '../../data/students';
import type { PulseAccount } from '../../identity';
import { findAssignedMentor } from '../../studentAccountLink.js';
import type { Language } from '../config';

// Keep the synthetic curriculum catalog in one place for student and cohort views.
export const projectCatalog = [
  { id: 'c-basics', name: { en: 'C foundations', ar: 'أساسيات C' } },
  { id: 'shell', name: { en: 'Shell navigation', ar: 'التعامل مع الطرفية' } },
  { id: 'git', name: { en: 'Git workflow', ar: 'إدارة الإصدارات' } },
  { id: 'loops', name: { en: 'Loops & conditions', ar: 'الحلقات والشروط' } },
  { id: 'functions', name: { en: 'Functions', ar: 'الدوال' } },
  { id: 'pointers', name: { en: 'Pointers', ar: 'المؤشرات' } },
  { id: 'arrays', name: { en: 'Arrays', ar: 'المصفوفات' } },
  { id: 'memory', name: { en: 'Memory management', ar: 'إدارة الذاكرة' } },
  { id: 'structures', name: { en: 'Data structures', ar: 'هياكل البيانات' } },
  { id: 'testing', name: { en: 'Testing & debugging', ar: 'الاختبار وتصحيح الأخطاء' } },
  { id: 'api', name: { en: 'API integration', ar: 'ربط واجهات البرمجة' } },
  { id: 'final', name: { en: 'Final project', ar: 'المشروع النهائي' } },
];

const mentorPalette = ['#168f79', '#795cb1', '#ba7934', '#3979a8', '#b95778', '#65843b'];

// Produce stable demo completion data until the curriculum API provides project records.
export function completedCount(student: StudentPaceRow) {
  return (Math.abs(student.studentId * 3 + student.actualLevel * 2) % (projectCatalog.length + 1));
}

function studentInitials(student: StudentPaceRow) {
  return (student.displayName || student.login).split(/\s+/).slice(0, 2).map(part => part[0]).join('').toUpperCase();
}

type ProjectProgressProps = { rows: StudentPaceRow[]; accounts: PulseAccount[]; mentors: PulseAccount[]; lang: Language; view: 'cohort' | 'student'; onSelectStudent?: (student: StudentPaceRow) => void };

export function ProjectProgress(props: ProjectProgressProps) {
  if (usesLiveApi()) return <ArchiveProjectProgress {...props} />;
  return <DemoProjectProgress {...props} />;
}

function ArchiveProjectProgress({ rows, lang, view, onSelectStudent }: ProjectProgressProps) {
  const ar = lang === 'ar';
  const studentId = view === 'student' ? rows[0]?.studentId : undefined;
  const [board, setBoard] = useState<ProgressBoard | null>(null);
  const [failed, setFailed] = useState(false);
  useEffect(() => {
    let live = true;
    setBoard(null);
    setFailed(false);
    const load = studentId ? getStudentProgress(studentId) : getCohortProgress();
    load.then(next => { if (live) setBoard(next); }).catch(() => { if (live) setFailed(true); });
    return () => { live = false; };
  }, [studentId]);
  const students = board?.students ?? [];
  const projects = board?.projects ?? [];
  const doneCells = students.reduce((sum, student) => sum + student.completed, 0);
  const totalCells = students.reduce((sum, student) => sum + student.total, 0);
  const completionRate = totalCells ? Math.round(doneCells / totalCells * 100) : 0;
  const visible = view === 'student' ? students.filter(student => student.studentId === studentId) : students;
  return <section className={`table-panel project-progress ${view === 'cohort' ? 'project-progress-cohort' : 'project-progress-student'}`}>
    <header className="project-progress-heading"><div><span className="eyebrow">{ar ? 'تقدم المشاريع' : 'PROJECT PROGRESS'}</span><h2>{view === 'cohort' ? (ar ? 'إنجاز مشاريع الكوهورت' : 'Cohort project progress') : (ar ? 'مشاريعك' : 'Your projects')}</h2><p>{board?.syncedAt ? (ar ? 'المربع يُملأ من سجل 01Edu المباشر.' : 'A square is filled from the live 01Edu record.') : (ar ? 'المربع يُملأ من أرشيف 01Edu إلى أن يحدّثه منتور أو مدير البرنامج.' : 'A square is filled from the 01Edu archive until a mentor or program manager signs in and refreshes it.')}</p></div><span className="project-demo-tag"><i />{board?.syncedAt ? (ar ? '01Edu مباشر' : 'Live 01Edu') : '01Edu'}</span></header>
    {failed && <p className="project-empty">{ar ? 'تعذر تحميل تقدم المشاريع من الأرشيف.' : 'Project progress could not be loaded from the archive.'}</p>}
    {!failed && !board && <p className="project-empty">{ar ? 'جارٍ التحميل من 01Edu…' : 'Loading project records from 01Edu…'}</p>}
    {board && projects.length === 0 && <p className="project-empty">{ar ? 'لا توجد مشاريع من النوع project في أرشيف 01Edu لهذا الكوهورت.' : 'The 01Edu archive has no project records for this cohort.'}</p>}
    {board && projects.length > 0 && <>
      <div className="project-overall-progress"><div><b>{completionRate}%</b><span>{ar ? `${doneCells} من ${totalCells} خلية منتهية` : `${doneCells} of ${totalCells} project cells are done`}</span></div><div className="project-overall-track"><i style={{ width: `${completionRate}%` }} /></div></div>
      <details className="project-key"><summary>{ar ? `أسماء المشاريع (${projects.length})` : `Project names (${projects.length})`}</summary><div>{projects.map((project, index) => <span key={project.id}><b>{`P${index + 1}`}</b>{project.name}</span>)}</div></details>
      <div className="project-student-board" aria-label={ar ? 'إنجاز المشاريع حسب الطالب' : 'Project completion by student'}>
        {visible.map(student => {
          const pace = rows.find(row => row.studentId === student.studentId);
          return <article className="project-student-card" key={student.studentId}>
            <header><span className="project-student-avatar">{(student.displayName || student.login).slice(0, 2).toUpperCase()}</span><span className="project-card-identity"><b>{student.displayName || student.login}</b><small>{student.mentorName || (ar ? 'لم يُسنَد' : 'Unassigned')}</small></span><span className="project-card-count"><b>{student.completed}<small>/{student.total}</small></b><span>{ar ? 'مشاريع مكتملة' : 'projects done'}</span></span></header>
            <div className="project-student-meter"><i style={{ width: `${student.total ? Math.round(student.completed / student.total * 100) : 0}%` }} /></div>
            <div className="project-card-steps">{projects.map((project, index) => { const complete = student.done[index]; return <div className="project-step" key={project.id} title={`${project.name}: ${complete ? (ar ? 'مكتمل' : 'Done') : (ar ? 'غير مكتمل' : 'Not done')}`}><span className={`project-square ${complete ? 'complete' : ''}`}>{complete ? '✓' : ''}</span><small>{`P${index + 1}`}</small></div>; })}</div>
            {view === 'cohort' && pace && onSelectStudent && <button type="button" className="setting-action" onClick={() => onSelectStudent(pace)}>{ar ? 'فتح السجل' : 'Open record'}</button>}
          </article>;
        })}
      </div>
    </>}
  </section>;
}

function DemoProjectProgress({ rows, accounts, mentors, lang, view, onSelectStudent }: ProjectProgressProps) {
  const ar = lang === 'ar';
  const isCohort = view === 'cohort';
  const [selection, setSelection] = useState<{ type: 'project'; index: number } | { type: 'mentor'; id: string } | null>(null);
  const projects = projectCatalog;
  const completedTotal = rows.reduce((total, student) => total + completedCount(student), 0);
  const totalCells = rows.length * projects.length;
  const completionRate = totalCells ? Math.round(completedTotal / totalCells * 100) : 0;
  const mentorColors = new Map<string, string>();
  rows.forEach(student => {
    const mentor = findAssignedMentor(student, accounts, mentors);
    const id = mentor?.id ?? 'unassigned';
    if (!mentorColors.has(id)) mentorColors.set(id, mentorPalette[mentorColors.size % mentorPalette.length]);
  });
  const cohortMentors = [...new Map(rows.map(student => findAssignedMentor(student, accounts, mentors)).filter((mentor): mentor is PulseAccount => Boolean(mentor)).map(mentor => [mentor.id, mentor])).values()];
  const projectRates = projects.map((project, index) => ({ project, index, count: rows.filter(student => completedCount(student) > index).length }));
  const mentorStats = [...new Map(rows.map(student => { const mentor = findAssignedMentor(student, accounts, mentors); return [mentor?.id ?? 'unassigned', { mentor, rows: [] as StudentPaceRow[] }] as const; })).values()]
    .map(group => ({ ...group, rows: rows.filter(student => (findAssignedMentor(student, accounts, mentors)?.id ?? 'unassigned') === (group.mentor?.id ?? 'unassigned')) }))
    .map(group => ({ ...group, done: group.rows.reduce((sum, student) => sum + completedCount(student), 0), average: group.rows.length ? Math.round(group.rows.reduce((sum, student) => sum + completedCount(student), 0) / group.rows.length / projects.length * 100) : 0 }));
  const selectedStudents = selection?.type === 'project'
    ? rows.filter(student => completedCount(student) <= selection.index)
    : selection?.type === 'mentor'
      ? rows.filter(student => (findAssignedMentor(student, accounts, mentors)?.id ?? 'unassigned') === selection.id)
      : [];
  const attentionRows = rows.map(student => ({ student, done: completedCount(student), remaining: projects.length - completedCount(student) })).filter(entry => entry.remaining > 0).sort((a, b) => b.remaining - a.remaining || a.done - b.done).slice(0, 5);

  return <section className={`table-panel project-progress ${isCohort ? 'project-progress-cohort' : 'project-progress-student'}`}>
    <header className="project-progress-heading"><div><span className="eyebrow">{ar ? 'تقدم المشاريع' : 'PROJECT PROGRESS'}</span><h2>{isCohort ? (ar ? 'إنجاز مشاريع الكوهورت' : 'Cohort project progress') : (ar ? 'مشاريعك' : 'Your projects')}</h2><p>{isCohort ? (ar ? 'كل مربع يمثل مشروعًا؛ المربع الملوّن مكتمل والفارغ لم يكتمل بعد.' : 'Each square is one project. Filled means complete; empty means not completed yet.') : (ar ? 'المربع الملوّن يعني أن المشروع اكتمل، والفارغ يعني أنه ما زال قيد العمل.' : 'A filled square means the project is complete; an empty one is still in progress.')}</p></div><span className="project-demo-tag"><i />{ar ? 'بيانات تجريبية' : 'Demo data'}</span></header>
    {isCohort ? <>
      <div className="project-overall-progress"><div><b>{completionRate}%</b><span>{ar ? `متوسط إنجاز ${rows.length} طالب عبر ${projects.length} مشروعًا` : `Average completion across ${rows.length} students and ${projects.length} projects`}</span></div><div className="project-overall-track"><i style={{ width: `${completionRate}%` }} /></div></div>
      <details className="project-key"><summary>{ar ? `أسماء المشاريع (${projects.length})` : `Project names (${projects.length})`}</summary><div>{projects.map((project, index) => <span key={project.id}><b>{`P${index + 1}`}</b>{project.name[lang]}</span>)}</div></details>
      <div className="project-student-board" aria-label={ar ? 'إنجاز المشاريع حسب الطالب' : 'Project completion by student'}>
        {rows.map(student => {
          const mentor = findAssignedMentor(student, accounts, mentors);
          const mentorId = mentor?.id ?? 'unassigned';
          const done = completedCount(student);
          return <article className="project-student-card" key={student.studentId}>
            <header><span className="project-student-avatar">{studentInitials(student)}</span><span className="project-card-identity"><b>{student.displayName || student.login}</b><small><i style={{ background: mentorColors.get(mentorId) }} />{mentor?.name ?? (ar ? 'غير مسند' : 'Unassigned')}</small></span><span className="project-card-count"><b>{done}<small>/{projects.length}</small></b><span>{ar ? 'مشاريع مكتملة' : 'projects done'}</span></span></header>
            <div className="project-student-meter"><i style={{ width: `${Math.round(done / projects.length * 100)}%` }} /></div>
            <div className="project-card-steps">{projects.map((project, index) => { const complete = index < done; return <div className="project-step" key={project.id} title={`${project.name[lang]}: ${complete ? (ar ? 'مكتمل' : 'Completed') : (ar ? 'لم يكتمل' : 'Not completed')}`}><span className={`project-square ${complete ? 'complete' : ''}`}>{complete ? '✓' : ''}</span><small>{`P${index + 1}`}</small></div>; })}</div>
          </article>;
        })}
        {rows.length === 0 && <p className="project-empty">{ar ? 'لا يوجد طلاب لعرض تقدمهم.' : 'No students are available.'}</p>}
      </div>
      <div className="project-mentor-legend">{cohortMentors.map(mentor => <span key={mentor.id}><i style={{ background: mentorColors.get(mentor.id) }} />{mentor.name}</span>)}{mentorColors.has('unassigned') && <span><i style={{ background: mentorColors.get('unassigned') }} />{ar ? 'غير مسند' : 'Unassigned'}</span>}</div>
      <div className="project-insights-grid">
        <article className="project-insight-card"><header><span><b>{ar ? 'إكمال كل مشروع' : 'Completion by project'}</b><small>{ar ? 'اضغط على مشروع لمعرفة من لم يكمله' : 'Select a project to see who has not completed it'}</small></span><span className="project-insight-mark">↗</span></header><div className="project-analysis-bars">{projectRates.map(({ project, index, count }) => { const rate = rows.length ? Math.round(count / rows.length * 100) : 0; const selected = selection?.type === 'project' && selection.index === index; return <button type="button" key={project.id} className={selected ? 'selected' : ''} aria-pressed={selected} onClick={() => setSelection(selected ? null : { type: 'project', index })}><span><b>{project.name[lang]}</b><small>{count}/{rows.length}</small></span><span className="project-analysis-track"><i style={{ width: `${rate}%` }} /></span><strong>{rate}%</strong></button>; })}</div></article>
        <article className="project-insight-card"><header><span><b>{ar ? 'مسار إكمال المشاريع' : 'Project completion funnel'}</b><small>{ar ? 'عدد الطلاب الذين وصلوا لكل مشروع' : 'Students who completed each step in sequence'}</small></span><span className="project-insight-mark">⌁</span></header><div className="project-funnel">{projectRates.map(({ project, index, count }) => { const selected = selection?.type === 'project' && selection.index === index; const rate = rows.length ? Math.round(count / rows.length * 100) : 0; return <button type="button" key={project.id} aria-pressed={selected} className={selected ? 'selected' : ''} onClick={() => setSelection(selected ? null : { type: 'project', index })}><span>{`P${index + 1}`}</span><i><b style={{ width: `${rate}%` }} /></i><strong>{count}</strong></button>; })}</div></article>
        <article className="project-insight-card"><header><span><b>{ar ? 'إنجاز الطلاب حسب المنتور' : 'Progress by mentor'}</b><small>{ar ? 'متوسط نسبة المشاريع المكتملة' : 'Average share of projects completed'}</small></span><span className="project-insight-mark">♙</span></header><div className="project-mentor-analysis">{mentorStats.map(({ mentor, rows: mentorRows, average }) => { const id = mentor?.id ?? 'unassigned'; const selected = selection?.type === 'mentor' && selection.id === id; return <button type="button" key={id} aria-pressed={selected} className={selected ? 'selected' : ''} onClick={() => setSelection(selected ? null : { type: 'mentor', id })}><span><i style={{ background: mentorColors.get(id) }} /><b>{mentor?.name ?? (ar ? 'غير مسند' : 'Unassigned')}</b><small>{mentorRows.length} {ar ? 'طالب' : 'students'}</small></span><strong>{average}%</strong><span className="project-analysis-track"><i style={{ width: `${average}%` }} /></span></button>; })}{mentorStats.length === 0 && <p className="project-empty">{ar ? 'لا توجد بيانات.' : 'No data available.'}</p>}</div></article>
        <article className="project-insight-card"><header><span><b>{ar ? 'طلاب يحتاجون متابعة' : 'Students to check in with'}</b><small>{ar ? 'الأكثر في المشاريع غير المكتملة' : 'Students with the most unfinished projects'}</small></span><span className="project-insight-mark">!</span></header><div className="project-attention-list">{attentionRows.map(({ student, done, remaining }) => { const mentor = findAssignedMentor(student, accounts, mentors); return <button type="button" key={student.studentId} onClick={() => onSelectStudent?.(student)} disabled={!onSelectStudent}><span className="project-attention-avatar">{studentInitials(student)}</span><span><b>{student.displayName || student.login}</b><small>{mentor?.name ?? (ar ? 'غير مسند' : 'Unassigned')}</small></span><strong>{remaining}<small>{ar ? 'متبقي' : 'left'}</small></strong><i>{done}/{projects.length}</i></button>; })}{attentionRows.length === 0 && <p className="project-empty">{ar ? 'كل الطلاب أكملوا المشاريع.' : 'All students completed the projects.'}</p>}</div></article>
      </div>
      {selection && <article className="project-insight-drilldown"><header><div><b>{selection.type === 'project' ? (ar ? `لم يكملوا ${projects[selection.index].name[lang]}` : `Not completed: ${projects[selection.index].name[lang]}`) : (ar ? `طلاب ${mentorStats.find(group => (group.mentor?.id ?? 'unassigned') === selection.id)?.mentor?.name ?? 'غير مسند'}` : `Students assigned to ${mentorStats.find(group => (group.mentor?.id ?? 'unassigned') === selection.id)?.mentor?.name ?? 'Unassigned'}`)}</b><small>{selectedStudents.length} {ar ? 'طالب' : 'students'}</small></div><button type="button" onClick={() => setSelection(null)} aria-label={ar ? 'إغلاق' : 'Close'}>×</button></header><div>{selectedStudents.map(student => { const mentor = findAssignedMentor(student, accounts, mentors); return <button type="button" key={student.studentId} onClick={() => onSelectStudent?.(student)} disabled={!onSelectStudent}><span><b>{student.displayName || student.login}</b><small>{mentor?.name ?? (ar ? 'غير مسند' : 'Unassigned')}</small></span><strong>{completedCount(student)}/{projects.length}</strong></button>; })}{selectedStudents.length === 0 && <p>{ar ? 'لا يوجد طلاب ضمن هذا الاختيار.' : 'No students match this selection.'}</p>}</div></article>}
    </> : <><div className="project-overall-progress"><div><b>{rows[0] ? completedCount(rows[0]) : 0}/{projects.length}</b><span>{ar ? `${projects.length - (rows[0] ? completedCount(rows[0]) : 0)} مشروع متبقي` : `${projects.length - (rows[0] ? completedCount(rows[0]) : 0)} projects remaining`}</span></div><div className="project-overall-track"><i style={{ width: `${completionRate}%` }} /></div><strong>{completionRate}%</strong></div><div className="project-student-journey">{projects.map((project, index) => { const done = rows[0] ? index < completedCount(rows[0]) : false; return <article className={`project-journey-step ${done ? 'complete' : ''}`} key={project.id}><span className={`project-square ${done ? 'complete' : ''}`}>{done ? '✓' : ''}</span><small>{`P${index + 1}`}</small><b>{project.name[lang]}</b><span className="project-journey-status">{done ? (ar ? 'مكتمل' : 'Completed') : (ar ? 'لم يكتمل' : 'To do')}</span></article>; })}</div></>}
  </section>;
}
