import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { useTranslation } from 'react-i18next';
import { i18n } from '../../i18n';
import type { ProfileSnapshot, ProfilesService } from '../../desktop-contract';
import '../../styles/profiles.css';
import { GnomeSelect } from '../panels/GnomeSelect';
import { FeedbackPortal } from '../OperationFeedback';

const profileError = (code: string) => code === 'duplicate_profile_name' ? i18n.t('profiles.errors.duplicate_profile_name') : code;
type Operation = 'Create' | 'Rename' | 'Duplicate' | 'Delete';
export function ProfileControls({ service, scope, disabled, refresh, onBusy }: {
  service: Partial<ProfilesService>; scope: string; disabled: boolean;
  refresh(): Promise<void>; onBusy(busy: boolean): void;
}) {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<ProfileSnapshot>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ operation: Operation; id: string }>();
  const [name, setName] = useState('');
  const [hiddenNotice, setHiddenNotice] = useState('');
  const [hiddenError, setHiddenError] = useState('');
  const [hiddenMessage, setHiddenMessage] = useState('');
  useEffect(() => {
    setHiddenNotice(''); setHiddenError(''); setHiddenMessage('');
  }, [scope, service, disabled]);
  useEffect(() => {
    if (!snapshot?.Notice) return;
    const timer = window.setTimeout(() => setHiddenNotice(snapshot.Notice), 8000);
    return () => window.clearTimeout(timer);
  }, [snapshot?.Notice, scope]);
  useEffect(() => {
    setHiddenError('');
    if (!error) return;
    const timer = window.setTimeout(() => setHiddenError(error), 8000);
    return () => window.clearTimeout(timer);
  }, [error, scope]);
  useEffect(() => {
    setHiddenMessage('');
    if (!message) return;
    const timer = window.setTimeout(() => setHiddenMessage(message), 3000);
    return () => window.clearTimeout(timer);
  }, [message, scope]);
  const epoch = useRef(0);
  const lock = useRef(false);
  useEffect(() => {
    const token = ++epoch.current;
    lock.current = false; setBusy(false); onBusy(false);
    setSnapshot(undefined); setError(''); setMessage(''); setOpen(false); setDialog(undefined);
    if (!disabled && service.GetProfiles) {
      lock.current = true; setBusy(true);
      void service.GetProfiles().then(next => {
        if (epoch.current === token) { setSnapshot(next); setError(next.Error.Code); }
      }).catch(() => { if (epoch.current === token) setError(t('profiles.errors.loadFailed')); })
        .finally(() => { if (epoch.current === token) { lock.current = false; setBusy(false); } });
    }
    return () => { epoch.current++; };
  }, [service, scope, disabled]);

  const run = async (request: () => Promise<ProfileSnapshot>, switching = false) => {
    if (lock.current || disabled) return;
    const token = epoch.current;
    const current = () => token === epoch.current;
    lock.current = true; setBusy(true); onBusy(true); setError(''); setMessage('');
    try {
      const next = await request();
      if (!current()) return;
      setSnapshot(next);
      setError(switching && (!next.Applied || next.Error.Code)
        ? `${t('profiles.errors.partial')}${next.Error.Code ? ` (${next.Error.Code})` : ''}`
        : profileError(next.Error.Code));
      if (switching && next.Applied && !next.Error.Code) setMessage(t('profiles.applied'));
      if (!next.Error.Code) setDialog(undefined);
    } catch {
      if (current()) setError(switching ? t('profiles.errors.switchFailed') : t('profiles.errors.actionFailed'));
    } finally {
      if (current()) {
        if (switching) {
          try { await refresh(); } catch { if (current()) setError(t('profiles.errors.refreshFailed')); }
        }
        if (current()) { lock.current = false; setBusy(false); onBusy(false); }
      }
    }
  };
  const unavailable = disabled || busy || !snapshot || !snapshot.Profiles.length;
  const begin = (operation: Operation, id = '', initialName = '') => { setName(initialName); setDialog({ operation, id }); };
  const submit = () => {
    if (!dialog) return;
    const { operation, id } = dialog;
    const request = operation === 'Create' ? () => service.CreateProfile!(name.trim())
      : operation === 'Rename' ? () => service.RenameProfile!(id, name.trim())
      : operation === 'Duplicate' ? () => service.DuplicateProfile!(id, name.trim())
      : () => service.DeleteProfile!(id);
    void run(request);
  };
  return <section className="profile-controls" aria-label={t('profiles.region')} aria-busy={busy}>
    <span className="profile-selector-label">{t('profiles.selector')}</span>
    <GnomeSelect aria-label={t('profiles.selector')} value={snapshot?.ActiveID ?? ''} disabled={unavailable}
      placeholder={t('profiles.unavailable')}
      options={snapshot?.Profiles.map(profile => ({ value: profile.ID, label: profile.Name, direct: true })) ?? []}
      onChange={value => void run(() => service.SwitchProfile!(value), true)} />
    <button type="button" className="profile-manage" aria-label={t('profiles.manage')} title={t('profiles.manage')} aria-expanded={open} disabled={unavailable} onClick={() => setOpen(!open)}><span aria-hidden="true">⋯</span></button>
    {createPortal(<div className={`profile-overlays${open || dialog ? ' profile-overlays-open' : ''}`}>
    {open && <section className="profile-management" aria-label={t('profiles.managementRegion')}>
      <header className="profile-panel-header"><div><h2>{t('profiles.title')}</h2><p>{t('profiles.subtitle')}</p></div>
        <button type="button" aria-label={t('profiles.closeManagement')} disabled={busy} onClick={() => setOpen(false)}>×</button></header>
      <div className="profile-toolbar">
      <button className="profile-primary" disabled={busy} onClick={() => begin('Create')}>{t('profiles.create')}</button></div>
      <p className="profile-description">{t('profiles.createHint')}</p>
      {snapshot?.Profiles.map(profile => <div className="profile-row" key={profile.ID}>
        <div className="profile-identity"><strong>{profile.Name}</strong>{profile.ID === snapshot.ActiveID && <span className="profile-badge">{t('profiles.active')}</span>}</div>
        <div className="profile-row-actions">
        <button aria-label={t('profiles.renameLabel', { name: profile.Name })} disabled={busy} onClick={() => begin('Rename', profile.ID, profile.Name)}>{t('profiles.rename')}</button>
        <button aria-label={t('profiles.duplicateLabel', { name: profile.Name })} disabled={busy} onClick={() => begin('Duplicate', profile.ID, t('profiles.copyName', { name: profile.Name }))}>{t('profiles.duplicate')}</button>
        <button className="profile-danger" aria-label={t('profiles.deleteLabel', { name: profile.Name })} disabled={busy || profile.ID === snapshot.ActiveID || snapshot.Profiles.length === 1}
          aria-describedby="profile-delete-help" onClick={() => begin('Delete', profile.ID)}>{t('profiles.delete')}</button></div>
      </div>)}
      <p id="profile-delete-help">{t('profiles.deleteHelp')}</p>
    </section>}
    {dialog && <form role="dialog" aria-modal="false" aria-label={t(`profiles.dialog.${dialog.operation}`)} onSubmit={event => { event.preventDefault(); submit(); }}>
      <h2>{t(`profiles.dialog.${dialog.operation}`)}</h2>
      {error && <p role="alert">{error}</p>}
      {dialog.operation === 'Delete' ? <p>{t('profiles.deleteQuestion')}</p> :
        <label>{t('profiles.nameLabel')} <input autoFocus value={name} disabled={busy} onChange={event => setName(event.target.value)} /></label>}
      <div className="profile-form-actions"><button className="profile-primary" aria-label={t('profiles.confirmAction')} type="submit" disabled={busy || (dialog.operation !== 'Delete' && !name.trim())}>{dialog.operation === 'Delete' ? t('profiles.dialog.Delete') : t('profiles.save')}</button>
      <button type="button" aria-label={t('profiles.cancelAction')} disabled={busy} onClick={() => setDialog(undefined)}>{t('common.cancel')}</button></div>
    </form>}
    </div>, document.body)}
    <FeedbackPortal><div className="profile-toasts">
    {!open && !dialog && snapshot?.Notice && snapshot.Notice !== hiddenNotice && <div role="status">{snapshot.Notice} <button disabled={busy} onClick={() => void run(() => service.DismissProfileNotice!())}>{t('profiles.dismissNotice')}</button></div>}
    {!open && !dialog && error && error !== hiddenError && <p role="alert">{error}</p>}
    {!open && !dialog && message && message !== hiddenMessage && <p role="status">{message}</p>}
    </div></FeedbackPortal>
  </section>;
}
