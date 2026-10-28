import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { ProfileSnapshot, ProfilesService } from '../../desktop-contract';
import '../../styles/profiles.css';
import { GnomeSelect } from '../panels/GnomeSelect';
import { FeedbackPortal } from '../OperationFeedback';

const profileErrorMessages: Record<string, string> = {
  duplicate_profile_name: 'A profile with that name already exists. Choose a different name.',
};
const profileError = (code: string) => profileErrorMessages[code] ?? code;
type Operation = 'Create' | 'Rename' | 'Duplicate' | 'Delete';
export function ProfileControls({ service, scope, disabled, refresh, onBusy }: {
  service: Partial<ProfilesService>; scope: string; disabled: boolean;
  refresh(): Promise<void>; onBusy(busy: boolean): void;
}) {
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
      }).catch(() => { if (epoch.current === token) setError('Unable to load local profiles.'); })
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
        ? `Profile was not fully applied; mouse settings may be partial.${next.Error.Code ? ` (${next.Error.Code})` : ''}`
        : profileError(next.Error.Code));
      if (switching && next.Applied && !next.Error.Code) setMessage('Profile applied.');
      if (!next.Error.Code) setDialog(undefined);
    } catch {
      if (current()) setError(switching ? 'Profile switch failed; mouse settings may be partial.' : 'Profile action failed.');
    } finally {
      if (current()) {
        if (switching) {
          try { await refresh(); } catch { if (current()) setError('Unable to refresh mouse settings after profile switch.'); }
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
  return <section className="profile-controls" aria-label="Local profiles" aria-busy={busy}>
    <span className="profile-selector-label">Profile</span>
    <GnomeSelect aria-label="Profile" value={snapshot?.ActiveID ?? ''} disabled={unavailable}
      placeholder="Profiles unavailable"
      options={snapshot?.Profiles.map(profile => ({ value: profile.ID, label: profile.Name, direct: true })) ?? []}
      onChange={value => void run(() => service.SwitchProfile!(value), true)} />
    <button type="button" className="profile-manage" aria-label="Manage profiles" title="Manage profiles" aria-expanded={open} disabled={unavailable} onClick={() => setOpen(!open)}><span aria-hidden="true">⋯</span></button>
    {createPortal(<div className={`profile-overlays${open || dialog ? ' profile-overlays-open' : ''}`}>
    {open && <section className="profile-management" aria-label="Profile management">
      <header className="profile-panel-header"><div><h2>Mouse profiles</h2><p>Local settings for this mouse</p></div>
        <button type="button" aria-label="Close profile management" disabled={busy} onClick={() => setOpen(false)}>×</button></header>
      <div className="profile-toolbar">
      <button className="profile-primary" disabled={busy} onClick={() => begin('Create')}>Create profile</button></div>
      <p className="profile-description">Create from your current settings. Your mouse stays unchanged.</p>
      {snapshot?.Profiles.map(profile => <div className="profile-row" key={profile.ID}>
        <div className="profile-identity"><strong>{profile.Name}</strong>{profile.ID === snapshot.ActiveID && <span className="profile-badge">Active</span>}</div>
        <div className="profile-row-actions">
        <button aria-label={`Rename ${profile.Name}`} disabled={busy} onClick={() => begin('Rename', profile.ID, profile.Name)}>Rename</button>
        <button aria-label={`Duplicate ${profile.Name}`} disabled={busy} onClick={() => begin('Duplicate', profile.ID, `${profile.Name} copy`)}>Duplicate</button>
        <button className="profile-danger" aria-label={`Delete ${profile.Name}`} disabled={busy || profile.ID === snapshot.ActiveID || snapshot.Profiles.length === 1}
          aria-describedby="profile-delete-help" onClick={() => begin('Delete', profile.ID)}>Delete</button></div>
      </div>)}
      <p id="profile-delete-help">The active or last profile cannot be deleted. Switch to another profile first.</p>
    </section>}
    {dialog && <form role="dialog" aria-modal="false" aria-label={`${dialog.operation} profile`} onSubmit={event => { event.preventDefault(); submit(); }}>
      <h2>{dialog.operation} profile</h2>
      {error && <p role="alert">{error}</p>}
      {dialog.operation === 'Delete' ? <p>Delete this local profile? This cannot be undone.</p> :
        <label>Profile name <input autoFocus value={name} disabled={busy} onChange={event => setName(event.target.value)} /></label>}
      <div className="profile-form-actions"><button className="profile-primary" aria-label="Confirm profile action" type="submit" disabled={busy || (dialog.operation !== 'Delete' && !name.trim())}>{dialog.operation === 'Delete' ? 'Delete profile' : 'Save profile'}</button>
      <button type="button" aria-label="Cancel profile action" disabled={busy} onClick={() => setDialog(undefined)}>Cancel</button></div>
    </form>}
    </div>, document.body)}
    <FeedbackPortal><div className="profile-toasts">
    {!open && !dialog && snapshot?.Notice && snapshot.Notice !== hiddenNotice && <div role="status">{snapshot.Notice} <button disabled={busy} onClick={() => void run(() => service.DismissProfileNotice!())}>Dismiss profile notice</button></div>}
    {!open && !dialog && error && error !== hiddenError && <p role="alert">{error}</p>}
    {!open && !dialog && message && message !== hiddenMessage && <p role="status">{message}</p>}
    </div></FeedbackPortal>
  </section>;
}
