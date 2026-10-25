import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { ProfileSnapshot, ProfilesService } from '../../desktop-contract';
import '../../styles/profiles.css';

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
        : next.Error.Code);
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
    <label>Local profile <select aria-label="Local profile" value={snapshot?.ActiveID ?? ''} disabled={unavailable}
      onChange={event => void run(() => service.SwitchProfile!(event.target.value), true)}>
      {!snapshot && <option value="">Profiles unavailable</option>}
      {snapshot?.Profiles.map(profile => <option key={profile.ID} value={profile.ID}>{profile.Name}</option>)}
    </select></label>
    <button type="button" aria-expanded={open} disabled={unavailable} onClick={() => setOpen(!open)}>Manage profiles</button>
    {createPortal(<div className="profile-overlays">
    {open && <div className="profile-management" aria-label="Profile management">
      <button disabled={busy} onClick={() => begin('Create')}>Create profile</button>
      <p>Create clones the active local settings and activates the copy without changing the mouse. Duplicate stays inactive.</p>
      {snapshot?.Profiles.map(profile => <div key={profile.ID}>
        <span>{profile.Name}</span>
        <button disabled={busy} onClick={() => begin('Rename', profile.ID, profile.Name)}>Rename {profile.Name}</button>
        <button disabled={busy} onClick={() => begin('Duplicate', profile.ID, `${profile.Name} copy`)}>Duplicate {profile.Name}</button>
        <button disabled={busy || profile.ID === snapshot.ActiveID || snapshot.Profiles.length === 1}
          aria-describedby="profile-delete-help" onClick={() => begin('Delete', profile.ID)}>Delete {profile.Name}</button>
      </div>)}
      <p id="profile-delete-help">The active or last profile cannot be deleted. Switch to another profile first.</p>
    </div>}
    {dialog && <form role="dialog" aria-modal="false" aria-label={`${dialog.operation} profile`} onSubmit={event => { event.preventDefault(); submit(); }}>
      <h2>{dialog.operation} profile</h2>
      {error && <p role="alert">{error}</p>}
      {dialog.operation === 'Delete' ? <p>Delete this local profile? This cannot be undone.</p> :
        <label>Profile name <input autoFocus value={name} disabled={busy} onChange={event => setName(event.target.value)} /></label>}
      <button type="submit" disabled={busy || (dialog.operation !== 'Delete' && !name.trim())}>Confirm profile action</button>
      <button type="button" disabled={busy} onClick={() => setDialog(undefined)}>Cancel profile action</button>
    </form>}
    {!open && !dialog && snapshot?.Notice && <div role="status">{snapshot.Notice} <button disabled={busy} onClick={() => void run(() => service.DismissProfileNotice!())}>Dismiss profile notice</button></div>}
    {!open && !dialog && error && <p role="alert">{error}</p>}
    {!open && !dialog && message && <p role="status">{message}</p>}
    </div>, document.body)}
  </section>;
}
