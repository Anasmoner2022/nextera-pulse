import { useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import { authenticateStoredAccount } from '../accountPersistence.js';
import type { PulseAccount, PulseRole, PulseSession } from './types';
import { identityCopy } from './copy';
import { demoSession, defaultPermissions } from './defaults';
import { getStaff, getStudents, signInWithAcademyLogin, usesLiveApi } from '../data/students';

export function IdentityGate({ lang, onSignIn }: { lang: 'en' | 'ar'; onSignIn: (session: PulseSession) => void; }) {
  const ar = lang === 'ar', t = identityCopy[lang];
  const [theme, setTheme] = useState<'light' | 'dark'>(() => localStorage.getItem('pulse-theme') === 'dark' ? 'dark' : 'light');
  const logo = theme === 'dark' ? '/brand/nextera-wordmark-light.png' : '/brand/nextera-wordmark-dark.png';
  useEffect(() => {
    document.documentElement.lang = lang;
    document.documentElement.dir = ar ? 'rtl' : 'ltr';
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('pulse-theme', theme);
  }, [lang, ar, theme]);
  const [email, setEmail] = useState('');
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);
  const live = usesLiveApi();
  useEffect(() => {
    const page = document.querySelector<HTMLElement>('.auth-page');
    if (!page || !window.matchMedia('(pointer: fine)').matches || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    let pointerX = 0;
    let pointerY = 0;
    const move = (event: PointerEvent) => {
      pointerX = event.clientX;
      pointerY = event.clientY;
      page.querySelectorAll<HTMLElement>('.magnetic-item').forEach(item => {
        const rect = item.getBoundingClientRect();
        const dx = pointerX - (rect.left + rect.width / 2);
        const dy = pointerY - (rect.top + rect.height / 2);
        const distance = Math.hypot(dx, dy);
        const radius = 150;
        const pull = distance < radius ? (1 - distance / radius) * 13 : 0;
        item.style.setProperty('--pull-x', `${distance ? dx / distance * pull : 0}px`);
        item.style.setProperty('--pull-y', `${distance ? dy / distance * pull : 0}px`);
        item.classList.toggle('is-magnetic', pull > 1);
      });
    };
    const leave = () => {
      page.querySelectorAll<HTMLElement>('.magnetic-item').forEach(item => {
        item.style.setProperty('--pull-x', '0px');
        item.style.setProperty('--pull-y', '0px');
        item.classList.remove('is-magnetic');
      });
    };
    page.addEventListener('pointermove', move);
    page.addEventListener('pointerleave', leave);
    return () => {
      page.removeEventListener('pointermove', move);
      page.removeEventListener('pointerleave', leave);
    };
  }, []);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy(true); setNotice('');
    try {
      if (live) {
        const person = await signInWithAcademyLogin(login, password);
        onSignIn({
          id: `01edu-${person.pulseRole}-${person.studentId}`,
          name: person.displayName || person.login,
          email: person.email,
          role: person.pulseRole,
          permissions: defaultPermissions[person.pulseRole],
          studentLogin: person.login,
        });
        return;
      }
      const session = await authenticateStoredAccount<PulseAccount>(localStorage, email, password);
      if (!session) { setNotice(ar ? 'البريد الإلكتروني أو كلمة المرور غير صحيحين.' : 'The email or password is incorrect.'); return; }
      onSignIn(session as PulseSession);
    } catch (error) {
      const code = error instanceof Error ? error.message : '';
      const liveNotice = code === 'INVALID_CREDENTIALS'
        ? (ar ? 'اسم الدخول أو كلمة المرور غير صحيحة.' : 'The 01Edu login or password is incorrect.')
        : code === 'NOT_IN_COHORT'
          ? (ar ? 'هذا الحساب غير موجود في البيانات المحملة.' : 'This 01Edu account is not in the loaded cohort.')
          : code === 'SIGNIN_UNAVAILABLE'
            ? (ar ? 'تعذر الوصول إلى تسجيل 01Edu.' : 'The 01Edu sign-in service did not respond.')
            : (ar ? 'تعذر تسجيل الدخول. جرّب مرة أخرى.' : 'Could not sign in. Please try again.');
      setNotice(live ? liveNotice : (ar ? 'تعذر تسجيل الدخول. جرّب مرة أخرى.' : 'Could not sign in. Please try again.'));
    }
    finally { setBusy(false); }
  };
  const enterRole = async (role: PulseRole) => {
    if (!live) { onSignIn(demoSession(role)); return; }
    setBusy(true); setNotice('');
    try {
      const [staff, cohort] = await Promise.all([getStaff(), getStudents()]);
      const person = role === 'manager' ? staff.managers[0] : role === 'mentor' ? staff.mentors[0] : undefined;
      if (person) {
        onSignIn({ id: `01edu-${person.pulseRole}-${person.studentId}`, name: person.displayName || person.login, email: person.email, role: person.pulseRole, permissions: defaultPermissions[person.pulseRole], studentLogin: person.login });
        return;
      }
      const staffLogins = new Set([...staff.managers, ...staff.mentors].map(member => member.login.trim().replace(/^@/, '').toLowerCase()));
      const student = cohort.students.find(row => !staffLogins.has(row.login.trim().replace(/^@/, '').toLowerCase()));
      if (!student) { setNotice(ar ? 'لا توجد تجربة متاحة في البيانات المحملة.' : 'That workspace is not in the loaded cohort.'); return; }
      onSignIn({ id: `01edu-student-${student.studentId}`, name: student.displayName || student.login, email: student.email || '', role: 'student', permissions: defaultPermissions.student, studentLogin: student.login });
    } catch {
      setNotice(ar ? 'تعذر فتح التجربة. تأكد أن الخادم يعمل.' : 'Could not open that workspace. Check that the server is running.');
    } finally { setBusy(false); }
  };
  return <main className="auth-page" dir={ar ? 'rtl' : 'ltr'}>
    <div className="auth-glow glow-one" /><div className="auth-glow glow-two" />
    <div className="auth-x-watermark" aria-hidden="true" />
    <header className="auth-top"><a className="auth-brand" href="#"><img src={logo} alt="Nextera" /><span><b>PULSE</b><small>STUDENT SUCCESS</small></span></a><div className="auth-top-actions"><button className="auth-theme-toggle" onClick={() => setTheme(current => current === 'dark' ? 'light' : 'dark')} aria-label={theme === 'dark' ? (ar ? 'تفعيل الوضع الفاتح' : 'Switch to light mode') : (ar ? 'تفعيل الوضع الداكن' : 'Switch to dark mode')} title={theme === 'dark' ? (ar ? 'الوضع الفاتح' : 'Light mode') : (ar ? 'الوضع الداكن' : 'Dark mode')}>{theme === 'dark' ? '☀' : '☾'}</button></div></header>
    <section className="auth-layout"><div className="auth-story"><span className="auth-kicker"><i /> {ar ? 'منصة نجاح الطلاب' : 'STUDENT SUCCESS PLATFORM'}</span><h1>{ar ? 'كل خطوة في التعلّم لها نبض.' : 'Every learning journey has a pulse.'}</h1><p>{ar ? 'تابع التقدم، افهم التحديات، وخلّي الخطوة الجاية أوضح لكل طالب.' : 'See progress clearly, understand challenges, and make the next step count.'}</p><div className="learning-visual" aria-label={ar ? 'مسار تعلم البرمجة' : 'A programming learning path'}>
        <svg className="learning-path" viewBox="0 0 440 220" fill="none" aria-hidden="true"><path d="M49 145C105 145 103 66 166 66S229 161 286 161 337 94 397 94"/><path className="path-progress" d="M49 145C105 145 103 66 166 66S229 161 286 161 337 94 397 94"/></svg>
        <div className="learning-console"><div className="console-top"><i/><i/><i/><span>learning-path.js</span></div><div className="console-code"><span>01</span> <b>const</b> journey = <em>[</em><br/><span>02</span> &nbsp; <strong>learn</strong>, build, grow<br/><span>03</span> <em>]</em>;<i className="console-caret"/></div><div className="console-progress"><i/></div></div>
        <div className="path-node node-learn magnetic-item"><span>01</span><b>{ar ? 'تعلّم' : 'LEARN'}</b></div><div className="path-node node-build magnetic-item"><span>{'</>'}</span><b>{ar ? 'طبّق' : 'BUILD'}</b></div><div className="path-node node-grow magnetic-item"><span>↗</span><b>{ar ? 'تقدّم' : 'GROW'}</b></div>
      </div></div>
      <div className="auth-card"><span className="auth-card-mark">✦</span><div className="eyebrow">{ar ? 'أهلًا بك في Nextera Pulse' : 'WELCOME TO NEXTERA PULSE'}</div><h2>{t.signIn}</h2><p>{live ? (ar ? 'سجّل الدخول بنفس اسم الدخول وكلمة المرور في 01Edu.' : 'Sign in with the same 01Edu login and password.') : t.intro}</p>{live ? <form className="auth-form" onSubmit={submit}><label>{ar ? 'اسم الدخول في 01Edu' : '01Edu login'}<input autoComplete="username" value={login} onChange={event => setLogin(event.target.value)} placeholder="login" required /></label><label>{t.password}<input type="password" autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} required /></label><button className="primary-button auth-submit" type="submit" disabled={busy}>{busy ? '…' : t.continue} <span>→</span></button></form> : <form className="auth-form" onSubmit={submit}><label>{t.email}<input type="email" autoComplete="username" value={email} onChange={event => setEmail(event.target.value)} placeholder="name@example.com" required /></label><label>{t.password}<input type="password" autoComplete="current-password" value={password} onChange={event => setPassword(event.target.value)} required /></label><button className="primary-button auth-submit" type="submit" disabled={busy}>{busy ? '…' : t.continue} <span>→</span></button></form>}<div className="auth-divider"><span>{live ? (ar ? 'أو ادخل التجربة' : 'OR OPEN A WORKSPACE') : (ar ? 'أو جرّب النسخة' : 'OR PREVIEW THE DEMO')}</span></div><div className="demo-actions"><button type="button" onClick={() => enterRole('manager')} disabled={busy}>{t.demoManager} ↗</button><button type="button" onClick={() => enterRole('mentor')} disabled={busy}>{t.demoMentor} ↗</button><button type="button" onClick={() => enterRole('student')} disabled={busy}>{t.demoStudent} ↗</button></div>{!live && <button className="google-button" onClick={() => setNotice(t.googleNote)}><b className="google-g" aria-hidden="true">G</b><span>{t.google}</span></button>}{notice && <p className="auth-notice" role="status">{notice}</p>}{!live && <small className="auth-legal">{t.demoNote}</small>}</div></section>
    <footer className="auth-footer"><span>© 2026 Nextera Pulse</span><span>{ar ? 'مدعوم من Nextera × 01 Coding' : 'Powered by Nextera × 01 Coding'}</span></footer>
  </main>;
}
