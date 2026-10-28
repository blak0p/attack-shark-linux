import { useEffect, useRef, useState } from 'react';
import type { GamesService, ProfileSnapshot, ProfilesService, WatcherRule, WatcherService, WatcherSettingsSnapshot } from '../../desktop-contract';
import '../../styles/games.css';
import { GnomeSelect } from '../panels/GnomeSelect';
import { AddGameDialog } from './AddGameDialog';

const watcherErrors: Record<string, string> = {
  invalid_watcher_settings: 'That rule is not valid. Use the executable name only, without a path.',
  watcher_unavailable: 'Automatic switching is not available.',
};

/** Escapes a name for use inside double quotes in a POSIX shell. */
export const shellQuote = (name: string) => name.replace(/[\\"$`]/g, '\\$&');
export const launchCommand = (name: string) => `attack-shark-linux --profile "${shellQuote(name)}" -- %command%`;

export function GamesPanel({ service }: { service: Partial<WatcherService> & Partial<ProfilesService> & Partial<GamesService> }) {
  const [settings, setSettings] = useState<WatcherSettingsSnapshot>();
  const [profiles, setProfiles] = useState<ProfileSnapshot['Profiles']>([]);
  const [activeID, setActiveID] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [adding, setAdding] = useState(false);
  const [formProfile, setFormProfile] = useState('');
  const [commandProfile, setCommandProfile] = useState('');
  const [copy, setCopy] = useState<{ text: string; ok: boolean }>();
  const commandRef = useRef<HTMLElement>(null);
  const lock = useRef(false);

  useEffect(() => {
    let current = true;
    void service.GetWatcherSettings?.().then(next => {
      if (!current) return;
      setSettings(next); setError(watcherErrors[next.Error.Code] ?? next.Error.Code);
    }).catch(() => { if (current) setError(watcherErrors.watcher_unavailable); });
    void service.GetProfiles?.().then(next => {
      if (!current) return;
      setProfiles(next.Profiles); setActiveID(next.ActiveID);
    }).catch(() => { if (current) setError('Unable to load local profiles.'); });
    return () => { current = false; };
  }, [service]);

  const nameOf = (id: string) => profiles.find(profile => profile.ID === id)?.Name;
  const profileOptions = profiles.map(profile => ({ value: profile.ID, label: profile.Name, direct: true }));
  const defaultProfile = profiles.some(p => p.ID === activeID) ? activeID : profiles[0]?.ID ?? '';
  const selectedForm = profiles.some(p => p.ID === formProfile) ? formProfile : defaultProfile;
  const selectedCommand = profiles.some(p => p.ID === commandProfile) ? commandProfile : defaultProfile;
  const enabled = settings?.Enabled ?? false;
  const rules = settings?.Rules ?? [];

  const persist = async (nextEnabled: boolean, nextRules: WatcherRule[]) => {
    if (lock.current || !service.SetWatcherSettings) return false;
    lock.current = true; setBusy(true); setError('');
    try {
      const next = await service.SetWatcherSettings(nextEnabled, nextRules);
      setSettings(next);
      const code = next.Error.Code;
      setError(code ? watcherErrors[code] ?? code : '');
      return !code;
    } catch {
      setError(watcherErrors.watcher_unavailable);
      return false;
    } finally { lock.current = false; setBusy(false); }
  };
  const move = (index: number, delta: number) => {
    const next = [...rules];
    [next[index], next[index + delta]] = [next[index + delta], next[index]];
    void persist(enabled, next);
  };
  const addRule = async (rule: WatcherRule) => {
    const ok = await persist(enabled, [...rules, rule]);
    if (ok) setAdding(false);
    return ok;
  };
  const openDialog = () => { setError(''); setAdding(true); };
  const closeDialog = () => { setError(''); setAdding(false); };
  const command = launchCommand(nameOf(selectedCommand) ?? '');
  const copyCommand = async () => {
    try {
      if (!navigator.clipboard?.writeText) throw new Error('clipboard unavailable');
      await navigator.clipboard.writeText(command);
      setCopy({ text: 'Copied to the clipboard.', ok: true });
    } catch {
      if (commandRef.current) {
        const range = document.createRange(); range.selectNodeContents(commandRef.current);
        const selection = window.getSelection(); selection?.removeAllRanges(); selection?.addRange(range);
      }
      setCopy({ text: 'Press Ctrl+C to copy the selected command.', ok: false });
    }
  };
  const loaded = !!settings;
  const off = !enabled;
  const controlsDisabled = !loaded || busy;

  return <div className="games-panel">
    <section className="games-card">
      <div className="games-cardrow">
        <div><strong>Switch profile automatically</strong>
          <span className="help">Checks running programs every few seconds. Works while this app is open.</span></div>
        <input className="switch" type="checkbox" aria-label="Switch profile automatically" checked={enabled}
          disabled={!loaded || busy} onChange={event => void persist(event.target.checked, rules)} />
      </div>
      {loaded && off && <p className="help games-offnote">Automatic switching is off. Your rules are saved and will apply when you turn it on.</p>}
    </section>
    <section className="games-card" role="region" aria-labelledby="games-rules-title">
      <div className="games-cardrow">
        <div><strong id="games-rules-title">Rules</strong>
          <span className="help">If several games run, the first rule in the list wins.</span></div>
      </div>
      {rules.length === 0 ? (loaded && <div className="games-empty"><strong>No rules yet</strong>Add a game or app and choose its profile.</div>) :
        <table className="games-table"><thead><tr>
          <th scope="col">#</th><th scope="col">Name</th><th scope="col">Profile</th>
          <th scope="col"><span style={{ position: 'absolute', left: -9999 }}>Actions</span></th></tr></thead>
          <tbody>{rules.map((rule, index) => {
            const known = nameOf(rule.ProfileID) !== undefined;
            const primary = rule.Name || rule.Executable;
            const secondary = rule.Folder || rule.Executable;
            return <tr key={`${rule.Folder}\n${rule.Executable}`}>
              <td className="n">{index + 1}</td>
              <td><strong className="games-name">{primary}</strong>
                {secondary !== primary && <span className="games-sub">{secondary}</span>}</td>
              <td><span className={`games-profile-select${known ? '' : ' invalid'}`}>
                <GnomeSelect aria-label={`Profile for ${primary}`} value={rule.ProfileID} placeholder="Profile missing"
                  options={profileOptions} disabled={controlsDisabled}
                  onChange={value => void persist(enabled, rules.map((r, i) => i === index ? { ...r, ProfileID: value } : r))} />
              </span>{!known && loaded && <span className="help games-error">The profile for {primary} no longer exists. Choose another profile.</span>}</td>
              <td className="games-actions">
                <button type="button" className="order" aria-label={`Move ${primary} up`} disabled={controlsDisabled || index === 0} onClick={() => move(index, -1)}>↑</button>{' '}
                <button type="button" className="order" aria-label={`Move ${primary} down`} disabled={controlsDisabled || index === rules.length - 1} onClick={() => move(index, 1)}>↓</button>{' '}
                <button type="button" className="small danger" aria-label={`Remove ${primary}`} disabled={controlsDisabled}
                  onClick={() => void persist(enabled, rules.filter((_, i) => i !== index))}>Remove</button>
              </td></tr>;
          })}</tbody></table>}
      <div className="games-addbar">
        <button type="button" className="primary" disabled={controlsDisabled || !profiles.length} onClick={openDialog}>Add a game</button>
      </div>
      {adding && <AddGameDialog service={service} rules={rules} profileOptions={profileOptions} profile={selectedForm}
        error={error} onError={setError} onProfile={setFormProfile} onAdd={addRule} onClose={closeDialog} />}
    </section>
    <section className="games-card" aria-labelledby="games-launch-title">
      <div className="games-launch">
        <div className="games-launch-head">
          <div><strong id="games-launch-title">Without the app open</strong>
            <span className="help">For Steam and other launchers. Works even when this app is closed; if the app is open, it takes care of the mouse.</span></div>
          <GnomeSelect aria-label="Profile for the launch command" value={selectedCommand} placeholder="No profiles"
            options={profileOptions} disabled={!profiles.length} onChange={value => { setCommandProfile(value); setCopy(undefined); }} />
        </div>
        <div className="games-cmd">
          <code ref={commandRef} aria-label="Launch command">{command}</code>
          <button type="button" disabled={!profiles.length} onClick={() => void copyCommand()}>Copy</button>
        </div>
        <p className={`help games-status${copy?.ok ? ' ok' : ''}`} role="status">{copy?.text}</p>
        <ol className="games-steps"><li>In Steam, right-click the game and open Properties.</li><li>Paste the command into Launch Options.</li><li>The profile is applied when the game starts and restored when it closes.</li></ol>
      </div>
    </section>
    <p className="help games-error" role="alert" hidden={!error || adding}>{error}</p>
  </div>;
}
