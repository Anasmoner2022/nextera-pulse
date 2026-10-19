import { useEffect, useRef, useState } from 'react';
import type { StudentPaceRow } from '../../domain';
import { ProfileAvatar } from '../../identity';
import type { PulseAccount } from '../../identity';
import { normalizeLogin } from '../config';
import type { Language } from '../config';

type TeamProps = {
  mentors: PulseAccount[];
  rows: StudentPaceRow[];
  lang: Language;
  onUpdate: (mentor: PulseAccount) => void;
  onAssign: (mentorId: string, login: string, assigned: boolean) => void;
};

function MentorPicker({
  mentors,
  rows,
  current,
  student,
  lang,
  assignedTo,
  onAssign,
}: {
  mentors: PulseAccount[];
  rows: StudentPaceRow[];
  current: string;
  student: StudentPaceRow;
  lang: Language;
  assignedTo: (login: string) => string;
  onAssign: TeamProps['onAssign'];
}) {
  const ar = lang === 'ar';
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const selected = mentors.find(mentor => mentor.id === current);

  useEffect(() => {
    if (!open) return;
    const closeOnOutside = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', closeOnOutside);
    document.addEventListener('keydown', closeOnEscape);
    return () => {
      document.removeEventListener('mousedown', closeOnOutside);
      document.removeEventListener('keydown', closeOnEscape);
    };
  }, [open]);

  return <div className={`mentor-picker${open ? ' is-open' : ''}`} ref={rootRef}>
    <button
      className={`mentor-picker-trigger${!selected ? ' is-empty' : ''}`}
      type="button"
      aria-haspopup="listbox"
      aria-expanded={open}
      aria-label={`${student.displayName || student.login} ${ar ? 'المنتور المسؤول' : 'assigned mentor'}`}
      onClick={() => setOpen(value => !value)}
    >
      {selected ? <>
        <ProfileAvatar account={selected} size="small" />
        <span className="mentor-picker-selected"><b>{selected.name}</b><small>{selected.email}</small></span>
      </> : <span className="mentor-picker-placeholder">{ar ? 'اختر منتور' : 'Choose a mentor'}</span>}
      <span className="mentor-picker-chevron" aria-hidden="true">⌄</span>
    </button>
    {open && <div className="mentor-picker-menu" role="listbox" aria-label={ar ? 'المنتورز المتاحون' : 'Available mentors'}>
      {mentors.map(mentor => {
        const assigned = rows.filter(row => assignedTo(row.login) === mentor.id).length;
        const capacity = mentor.capacityLimit ?? Math.max(20, rows.length);
        const usedExcludingStudent = rows.filter(row => assignedTo(row.login) === mentor.id && normalizeLogin(row.login) !== normalizeLogin(student.login)).length;
        const full = usedExcludingStudent >= capacity;
        const isSelected = mentor.id === current;
        const percent = capacity > 0 ? Math.min(100, (assigned / capacity) * 100) : 0;
        return <button
          className="mentor-picker-option"
          type="button"
          role="option"
          aria-selected={isSelected}
          disabled={full && !isSelected}
          key={mentor.id}
          onClick={() => {
            onAssign(mentor.id, student.login, true);
            setOpen(false);
          }}
        >
          <ProfileAvatar account={mentor} size="small" />
          <span className="mentor-picker-option-info"><b>{mentor.name}</b><small>{mentor.email}</small><i><em style={{ width: `${percent}%` }} /></i></span>
          <span className={`mentor-picker-load${full && !isSelected ? ' is-full' : ''}`}>
            <b>{assigned}<small> / {capacity}</small></b>
            <small>{full && !isSelected ? (ar ? 'مكتمل' : 'Full') : (ar ? 'طالب' : 'students')}</small>
          </span>
          {isSelected && <span className="mentor-picker-check" aria-hidden="true">✓</span>}
        </button>;
      })}
      {!mentors.length && <p className="mentor-picker-empty">{ar ? 'لا يوجد منتورز متاحون' : 'No mentors available'}</p>}
    </div>}
  </div>;
}

export function TeamManagement({ mentors, rows, lang, onUpdate, onAssign }: TeamProps) {
  const ar = lang === 'ar';
  const [query, setQuery] = useState('');
  const assignedTo = (login: string) => mentors.find(mentor => (mentor.assignedStudentLogins ?? []).some(item => normalizeLogin(item) === normalizeLogin(login)))?.id ?? '';
  const visibleRows = rows.filter(student => `${student.displayName ?? ''} ${student.login}`.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));

  return <section className="table-panel team-management-panel">
    <header className="team-management-intro"><div><span className="eyebrow">{ar ? 'توزيع المجموعة' : 'TEAM & COHORT'}</span><h2>{ar ? 'إدارة المنتورز والطلاب' : 'Mentor management'}</h2><p>{ar ? 'نظّم سعة المنتورز وتابع إسناد كل طالب.' : 'Manage mentor capacity and keep every student assigned to one mentor.'}</p></div></header>
    <section className="team-mentor-section"><header><div><span className="eyebrow">{ar ? 'الفريق' : 'YOUR TEAM'}</span><h3>{ar ? 'المنتورز والسعة' : 'Mentors and capacity'}</h3></div><small>{ar ? 'عدّل الحد الأقصى للطلاب لكل منتور' : 'Set the maximum number of students per mentor'}</small></header>
      <div className="team-mentor-grid">{mentors.map(mentor => { const assigned = rows.filter(student => assignedTo(student.login) === mentor.id).length; const capacity = mentor.capacityLimit ?? Math.max(20, rows.length); const percent = capacity > 0 ? Math.min(100, (assigned / capacity) * 100) : 0; return <article className="team-mentor-card" key={mentor.id}><div className="team-mentor-card-head"><ProfileAvatar account={mentor} size="medium" /><div className="team-mentor-identity"><b>{mentor.name}</b><small>{mentor.email}</small></div><span className="role-pill mentor">{ar ? 'منتور' : 'Mentor'}</span></div><div className="team-capacity-summary"><span>{ar ? 'الطلاب المسندون' : 'Assigned students'}</span><b>{assigned}<small> / {capacity}</small></b></div><div className="team-capacity-track"><i style={{ width: `${percent}%` }} /></div><label className="capacity-editor">{ar ? 'الحد الأقصى للطلاب' : 'Student capacity'}<input aria-label={`${mentor.name} capacity`} type="number" min={assigned} max="500" value={capacity} onChange={event => onUpdate({ ...mentor, capacityLimit: Math.max(assigned, Number(event.target.value) || 0) })} /></label></article>;})}{!mentors.length && <div className="empty-state"><b>{ar ? 'لا يوجد منتورز في الفريق بعد' : 'No mentors in the team yet'}</b></div>}</div>
    </section>
    <section className="team-assignment-section"><header className="team-roster-header"><div><span className="eyebrow">{ar ? 'توزيع الطلاب' : 'STUDENT ROSTER'}</span><h3>{ar ? 'إسناد الطلاب' : 'Student assignments'}</h3></div><div className="team-roster-tools"><small>{ar ? `عرض ${visibleRows.length} من ${rows.length} طالب` : `Showing ${visibleRows.length} of ${rows.length} students`}</small><label className="team-roster-search"><span aria-hidden="true">⌕</span><input type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder={ar ? 'ابحث باسم الطالب أو الـ login' : 'Search name or login'} aria-label={ar ? 'ابحث عن طالب' : 'Search students'} />{query && <button type="button" onClick={() => setQuery('')} aria-label={ar ? 'مسح البحث' : 'Clear search'}>×</button>}</label></div></header>
      <div className="team-assignment-list">{visibleRows.map(student => { const index = rows.indexOf(student); const current = assignedTo(student.login); return <article className="team-assignment-row" key={student.studentId}><span className="team-assignment-index">{String(index + 1).padStart(2, '0')}</span><div className="team-assignment-student"><b>{student.displayName || student.login}</b><small>@{student.login}</small></div><span className={`team-assignment-status ${current ? 'assigned' : 'unassigned'}`}><i />{current ? (ar ? 'تم الإسناد' : 'Assigned') : (ar ? 'مطلوب إسناد' : 'Needs mentor')}</span><label className="team-assignment-control"><small>{ar ? 'المنتور المسؤول' : 'Assigned mentor'}</small><MentorPicker mentors={mentors} rows={rows} current={current} student={student} lang={lang} assignedTo={assignedTo} onAssign={onAssign} /></label></article>;})}{rows.length > 0 && visibleRows.length === 0 && <div className="team-roster-empty"><b>{ar ? 'ملقيناش طالب بالاسم ده' : 'No students match your search'}</b><span>{ar ? 'جرّب اسمًا أو login مختلفًا.' : 'Try another name or login.'}</span></div>}{!rows.length && <div className="empty-state"><b>{ar ? 'لا توجد بيانات طلاب' : 'No student records yet'}</b></div>}</div>
    </section>
    <p className="settings-note">{ar ? 'تغييرات السعة والتوزيع محفوظة في نسخة العرض على هذا الجهاز.' : 'Capacity and assignment changes are saved in this browser demo.'}</p>
  </section>;
}
