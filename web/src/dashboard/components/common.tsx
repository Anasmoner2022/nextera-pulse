import type { Language } from '../config';

import type { ReactNode } from 'react';

export function NavItem({ icon, label, active, onClick }: { icon: string; label: string; active: boolean; onClick: () => void ;}) { return <button className={`nav-item ${active ? 'active' : ''}`} onClick={onClick}><span>{icon}</span><span className="nav-label">{label}</span></button> ;}

export function SettingRow({ title, description, action }: { title: string; description: string; action: ReactNode ;}) { return <div className="setting-row"><div><b>{title}</b><p>{description}</p></div>{action}</div> ;}

export function InfoDialog({ title, body, onClose, closeLabel }: { title: string; body: string; onClose: () => void; closeLabel: string ;}) { return <div className="detail-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) onClose() ;}}><section className="info-dialog" role="dialog" aria-modal="true" aria-labelledby="info-title"><button className="detail-close" onClick={onClose} aria-label={closeLabel}>×</button><h2 id="info-title">{title}</h2><p>{body}</p><button className="primary-button" onClick={onClose}>{closeLabel}</button></section></div> ;}

export function NoAccess({ lang }: { lang: Language ;}) { return <section className="table-panel access-empty"><span>⌁</span><h2>{lang === 'ar' ? 'المساحة دي مش ضمن صلاحيات حسابك' : 'This area is not included in your access'}</h2><p>{lang === 'ar' ? 'اطلب من المنتور المسؤول تعديل صلاحيات حسابك.' : 'Ask your mentor to update your account permissions.'}</p></section> ;}
