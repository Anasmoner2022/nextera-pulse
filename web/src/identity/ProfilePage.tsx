import { useRef, useState } from 'react';
import type { ChangeEvent, PointerEvent as ReactPointerEvent } from 'react';
import { createPortal } from 'react-dom';
import type { PulseAccount, PulseSession } from './types';
import { identityCopy } from './copy';

type CropImage = { src: string; width: number; height: number };
type CropOffset = { x: number; y: number };
const CROP_SIZE = 280;

export function ProfileAvatar({ account, size = 'large' }: { account: Pick<PulseAccount, 'name' | 'avatar'>; size?: 'small' | 'medium' | 'large'; }) {
  return account.avatar
    ? <img className={`profile-avatar ${size}`} src={account.avatar} alt={account.name} />
    : <span className={`profile-avatar avatar-fallback ${size}`}>{account.name.split(/\s+/).slice(0, 2).map(part => part[0]).join('').toUpperCase()}</span>;
}

function readImage(file: File): Promise<CropImage> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error('Could not read this image.'));
    reader.onload = () => {
      if (typeof reader.result !== 'string') return reject(new Error('Could not read this image.'));
      const image = new Image();
      image.onload = () => resolve({ src: reader.result as string, width: image.naturalWidth, height: image.naturalHeight });
      image.onerror = () => reject(new Error('This image could not be opened.'));
      image.src = reader.result;
    };
    reader.readAsDataURL(file);
  });
}

export function ProfilePage({ account, viewer, lang, onSave, readOnly = false }: { account: PulseAccount; viewer: PulseSession; lang: 'en' | 'ar'; onSave: (account: PulseAccount) => boolean | void; readOnly?: boolean; }) {
  const ar = lang === 'ar', t = identityCopy[lang], canEdit = !readOnly && (viewer.id === account.id || viewer.permissions.includes('manageAccounts'));
  const [quote, setQuote] = useState(account.quote ?? '');
  const [notice, setNotice] = useState('');
  const [uploading, setUploading] = useState(false);
  const [cropImage, setCropImage] = useState<CropImage | null>(null);
  const [cropZoom, setCropZoom] = useState(1);
  const [cropOffset, setCropOffset] = useState<CropOffset>({ x: 0, y: 0 });
  const fileInput = useRef<HTMLInputElement>(null);
  const drag = useRef<{ pointerId: number; x: number; y: number } | null>(null);

  const save = (patch: Partial<PulseAccount>): boolean => {
    const updated = { ...account, quote, ...patch };
    if (onSave(updated) === false) return false;
    setNotice(t.profileSaved);
    return true;
  };

  const upload = async (event: ChangeEvent<HTMLInputElement>) => {
    const input = event.currentTarget, file = input.files?.[0];
    if (!file) return;
    input.value = '';
    if (!file.type.startsWith('image/') || file.size > 8_000_000) {
      setNotice(ar ? 'اختار صورة أقل من 8 ميجابايت.' : 'Choose an image under 8 MB.');
      return;
    }
    try {
      const image = await readImage(file);
      setCropImage(image);
      setCropZoom(1);
      setCropOffset({ x: 0, y: 0 });
      setNotice('');
    } catch (error) {
      setNotice(error instanceof Error ? error.message : (ar ? 'تعذر فتح الصورة.' : 'Could not open this image.'));
    }
  };

  const imageScale = cropImage ? Math.max(CROP_SIZE / cropImage.width, CROP_SIZE / cropImage.height) * cropZoom : 1;
  const renderedWidth = cropImage ? cropImage.width * imageScale : CROP_SIZE;
  const renderedHeight = cropImage ? cropImage.height * imageScale : CROP_SIZE;
  const limitOffset = (value: CropOffset, scale = imageScale): CropOffset => {
    if (!cropImage) return value;
    const maxX = Math.max(0, (cropImage.width * scale - CROP_SIZE) / 2);
    const maxY = Math.max(0, (cropImage.height * scale - CROP_SIZE) / 2);
    return { x: Math.max(-maxX, Math.min(maxX, value.x)), y: Math.max(-maxY, Math.min(maxY, value.y)) };
  };
  const startDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if ((event.target as HTMLElement).closest('input,button')) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    drag.current = { pointerId: event.pointerId, x: event.clientX, y: event.clientY };
  };
  const moveCrop = (event: ReactPointerEvent<HTMLDivElement>) => {
    const previous = drag.current;
    if (!previous || previous.pointerId !== event.pointerId) return;
    const dx = event.clientX - previous.x, dy = event.clientY - previous.y;
    drag.current = { pointerId: event.pointerId, x: event.clientX, y: event.clientY };
    setCropOffset(current => limitOffset({ x: current.x + dx, y: current.y + dy }));
  };
  const endDrag = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (drag.current?.pointerId === event.pointerId) drag.current = null;
  };

  const saveCrop = () => {
    if (!cropImage) return;
    setUploading(true);
    try {
      const sourceSize = CROP_SIZE / imageScale;
      const sourceX = Math.max(0, Math.min(cropImage.width - sourceSize, (cropImage.width - sourceSize) / 2 - cropOffset.x / imageScale));
      const sourceY = Math.max(0, Math.min(cropImage.height - sourceSize, (cropImage.height - sourceSize) / 2 - cropOffset.y / imageScale));
      const canvas = document.createElement('canvas');
      canvas.width = 512;
      canvas.height = 512;
      const context = canvas.getContext('2d');
      if (!context) throw new Error('Image editing is unavailable.');
      const image = new Image();
      image.onload = () => {
        context.drawImage(image, sourceX, sourceY, sourceSize, sourceSize, 0, 0, canvas.width, canvas.height);
        if (save({ avatar: canvas.toDataURL('image/jpeg', 0.82) })) setCropImage(null);
        setUploading(false);
      };
      image.onerror = () => {
        setUploading(false);
        setNotice(ar ? 'تعذر تجهيز الصورة.' : 'Could not prepare this image.');
      };
      image.src = cropImage.src;
    } catch (error) {
      setUploading(false);
      setNotice(error instanceof Error ? error.message : (ar ? 'تعذر حفظ الصورة.' : 'Could not save this image.'));
    }
  };

  return <section className="profile-page">
    <header className="profile-hero">
      <div className="profile-cover-orbit cover-orbit-one" />
      <div className="profile-cover-orbit cover-orbit-two" />
      <div className="profile-identity">
        <div className="profile-avatar-wrap">
          <ProfileAvatar account={account} />
          {canEdit && <button className="photo-edit" onClick={() => fileInput.current?.click()} aria-label={t.changePhoto} disabled={uploading}>＋</button>}
          <input ref={fileInput} type="file" accept="image/*" hidden onChange={upload} />
        </div>
        <div>
          <span className="eyebrow">{account.role === 'manager' ? (ar ? 'إدارة البرنامج' : 'PROGRAM MANAGER') : account.role === 'mentor' ? (ar ? 'مساحة المنتور' : 'MENTOR PROFILE') : (ar ? 'مساحة الطالب' : 'STUDENT PROFILE')}</span>
          <h2>{account.name}</h2>
          <p>{account.email}</p>
        </div>
        <span className={'role-pill ' + account.role}>{account.role === 'manager' ? (ar ? 'مدير' : 'Manager') : account.role === 'mentor' ? t.mentor : t.student}</span>
      </div>
      {canEdit && <div className="profile-controls">
        <button className="setting-action" onClick={() => fileInput.current?.click()} disabled={uploading}>{uploading ? (ar ? 'جارٍ تجهيز الصورة…' : 'Preparing photo…') : t.changePhoto}</button>
        {account.avatar && <button className="setting-action" onClick={() => save({ avatar: '' })} disabled={uploading}>{t.removePhoto}</button>}
      </div>}
    </header>

    <div className="profile-columns profile-columns-simple">
      <article className="table-panel profile-about">
        <header><div><span className="eyebrow">{ar ? 'اقتباسك' : 'YOUR QUOTE'}</span><h3>{t.profileTitle}</h3></div>{canEdit && <button className="primary-button" onClick={() => save({})}>{t.saveProfile}</button>}</header>
        {canEdit ? <label>{t.quote}<input value={quote} onChange={event => setQuote(event.target.value)} placeholder={t.quoteHint} maxLength={120} /></label> : <blockquote>{quote ? '“' + quote + '”' : t.profileHint}</blockquote>}
      </article>
      <aside className="profile-side"><article className="table-panel profile-quote"><span>“</span><p>{quote || t.profileHint}</p><small>— {account.name}</small></article></aside>
    </div>

    {cropImage && createPortal(<div className="detail-backdrop crop-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) setCropImage(null); }}>
      <section className="crop-dialog" role="dialog" aria-modal="true" aria-labelledby="crop-title" dir={lang === 'ar' ? 'rtl' : 'ltr'}>
        <header className="crop-dialog-header">
          <div><span className="eyebrow">{ar ? 'صورة الملف الشخصي' : 'PROFILE PHOTO'}</span><h2 id="crop-title">{ar ? 'ظبّط قص الصورة' : 'Adjust your photo'}</h2><p>{ar ? 'اسحب الصورة لاختيار الجزء اللي هيظهر.' : 'Drag the photo to choose what appears in your avatar.'}</p></div>
          <button className="detail-close" onClick={() => setCropImage(null)} aria-label={ar ? 'إلغاء' : 'Cancel'}>×</button>
        </header>
        <div className="crop-workspace">
          <div className="crop-frame" onPointerDown={startDrag} onPointerMove={moveCrop} onPointerUp={endDrag} onPointerCancel={endDrag}>
            <img src={cropImage.src} alt="" draggable={false} style={{ width: renderedWidth, height: renderedHeight, transform: `translate(calc(-50% + ${cropOffset.x}px), calc(-50% + ${cropOffset.y}px))` }} />
            <span className="crop-frame-hint">{ar ? 'معاينة دائرية' : 'CIRCULAR PREVIEW'}</span>
          </div>
          <div className="crop-controls">
            <label htmlFor="crop-zoom">{ar ? 'تكبير الصورة' : 'Zoom'}<span>{Math.round(cropZoom * 100)}%</span></label>
            <input id="crop-zoom" type="range" min="1" max="3" step="0.01" value={cropZoom} onChange={event => { const zoom = Number(event.target.value); setCropZoom(zoom); setCropOffset(current => limitOffset(current, Math.max(CROP_SIZE / cropImage.width, CROP_SIZE / cropImage.height) * zoom)); }} />
            <small>{ar ? 'اسحب الصورة أو استخدم شريط التكبير.' : 'Drag the image or use the zoom slider.'}</small>
          </div>
        </div>
        <footer className="crop-actions">
          <button className="setting-action" onClick={() => setCropImage(null)} disabled={uploading}>{ar ? 'إلغاء' : 'Cancel'}</button>
          <button className="primary-button" onClick={saveCrop} disabled={uploading}>{uploading ? (ar ? 'جارٍ الحفظ…' : 'Saving…') : (ar ? 'استخدام الصورة' : 'Use photo')}</button>
        </footer>
      </section>
    </div>, document.body)}
    {notice && <div className="identity-toast" role="status">{notice}</div>}
  </section>;
}
