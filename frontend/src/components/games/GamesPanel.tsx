import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { i18n } from '../../i18n';
import type { AutostartService, GamesService, ProfileSnapshot, ProfilesService, WatcherRule, WatcherService, WatcherSettingsSnapshot } from '../../desktop-contract';
import '../../styles/games.css';
import { GnomeSelect } from '../panels/GnomeSelect';
import { AddGameDialog } from './AddGameDialog';

const autostartError = () => i18n.t('games.errors.autostart');
const watcherErrors = (code: string): string | undefined =>
  code === 'invalid_watcher_settings' || code === 'watcher_unavailable' ? i18n.t(`games.errors.${code}`) : undefined;

/** Escapes a name for use inside double quotes in a POSIX shell. */
export const shellQuote = (name: string) => name.replace(/[\\"$`]/g, '\\$&');
/** Single-quotes a path for a POSIX shell, but only when it has unsafe characters. */
export const quoteLauncherPath = (path: string) =>
  /^[A-Za-z0-9_@%+=:,./-]+$/.test(path) ? path : `'${path.replace(/'/g, `'\\''`)}'`;
export const launchCommand = (name: string, launcher = 'attack-shark-linux') =>
  `${launcher === 'attack-shark-linux' ? launcher : quoteLauncherPath(launcher)} --profile "${shellQuote(name)}" -- %command%`;

export function GamesPanel({ service }: { service: Partial<WatcherService> & Partial<AutostartService> & Partial<ProfilesService> & Partial<GamesService> }) {
  const { t } = useTranslation();
  const [settings, setSettings] = useState<WatcherSettingsSnapshot>();
  const [profiles, setProfiles] = useState<ProfileSnapshot['Profiles']>([]);
  const [activeID, setActiveID] = useState('');
  const [loginStart, setLoginStart] = useState<boolean>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [adding, setAdding] = useState(false);
  const [formProfile, setFormProfile] = useState('');
  const [commandProfile, setCommandProfile] = useState('');
  const [launcherPath, setLauncherPath] = useState('');
  const [copy, setCopy] = useState<{ text: string; ok: boolean }>();
  const commandRef = useRef<HTMLElement>(null);
  const lock = useRef(false);

  useEffect(() => {
    let current = true;
    void service.GetWatcherSettings?.().then(next => {
      if (!current) return;
      setSettings(next); setError(watcherErrors(next.Error.Code) ?? next.Error.Code);
    }).catch(() => { if (current) setError(t('games.errors.watcher_unavailable')); });
    void service.GetAutostart?.().then(next => {
      if (current && !next.Error.Code) setLoginStart(next.Enabled);
    }).catch(() => {});
    void service.GetProfiles?.().then(next => {
      if (!current) return;
      setProfiles(next.Profiles); setActiveID(next.ActiveID);
    }).catch(() => { if (current) setError(t('games.errors.loadProfiles')); });
    void service.GetLauncherPath?.().then(next => {
      if (current && !next.Error.Code && next.Path) setLauncherPath(next.Path);
    }).catch(() => {});
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
      setError(code ? watcherErrors(code) ?? code : '');
      return !code;
    } catch {
      setError(t('games.errors.watcher_unavailable'));
      return false;
    } finally { lock.current = false; setBusy(false); }
  };
  const toggleTray = async (value: boolean) => {
    if (lock.current || !service.SetTrayEnabled) return;
    lock.current = true; setBusy(true); setError('');
    try {
      const next = await service.SetTrayEnabled(value);
      setSettings(next);
      const code = next.Error.Code;
      setError(code ? watcherErrors(code) ?? code : '');
    } catch {
      setError(t('games.errors.watcher_unavailable'));
    } finally { lock.current = false; setBusy(false); }
  };
  const toggleLoginStart = async (value: boolean) => {
    if (lock.current || !service.SetAutostartEnabled) return;
    lock.current = true; setBusy(true); setError('');
    try {
      const next = await service.SetAutostartEnabled(value);
      setLoginStart(next.Enabled);
      setError(next.Error.Code ? autostartError() : '');
    } catch {
      setError(autostartError());
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
  const command = launchCommand(nameOf(selectedCommand) ?? '', launcherPath || undefined);
  const copyCommand = async () => {
    try {
      if (!navigator.clipboard?.writeText) throw new Error('clipboard unavailable');
      await navigator.clipboard.writeText(command);
      setCopy({ text: t('games.copied'), ok: true });
    } catch {
      if (commandRef.current) {
        const range = document.createRange(); range.selectNodeContents(commandRef.current);
        const selection = window.getSelection(); selection?.removeAllRanges(); selection?.addRange(range);
      }
      setCopy({ text: t('games.copyManual'), ok: false });
    }
  };
  const loaded = !!settings;
  const off = !enabled;
  const controlsDisabled = !loaded || busy;

  return <div className="games-panel">
    <section className="games-card">
      <div className="games-cardrow">
        <div><strong>{t('games.autoTitle')}</strong>
          <span className="help">{t('games.autoHelp')}</span></div>
        <input className="switch" type="checkbox" aria-label={t('games.autoTitle')} checked={enabled}
          disabled={!loaded || busy} onChange={event => void persist(event.target.checked, rules)} />
      </div>
      {service.SetTrayEnabled && <div className="games-cardrow">
        <div><strong>{t('games.trayTitle')}</strong>
          <span className="help">{t('games.trayHelp')}</span></div>
        <input className="switch" type="checkbox" aria-label={t('games.trayTitle')} checked={settings?.Tray ?? false}
          disabled={!loaded || busy} onChange={event => void toggleTray(event.target.checked)} />
      </div>}
      {service.SetAutostartEnabled && <div className="games-cardrow">
        <div><strong>{t('games.loginTitle')}</strong>
          <span className="help">{t('games.loginHelp')}</span></div>
        <input className="switch" type="checkbox" aria-label={t('games.loginTitle')} checked={loginStart ?? false}
          disabled={!loaded || busy || loginStart === undefined} onChange={event => void toggleLoginStart(event.target.checked)} />
      </div>}
      {loaded && off && <p className="help games-offnote">{t('games.offNote')}</p>}
    </section>
    <section className="games-card" role="region" aria-labelledby="games-rules-title">
      <div className="games-cardrow">
        <div><strong id="games-rules-title">{t('games.rulesTitle')}</strong>
          <span className="help">{t('games.rulesHelp')}</span></div>
      </div>
      {rules.length === 0 ? (loaded && <div className="games-empty"><strong>{t('games.emptyTitle')}</strong>{t('games.emptyHelp')}</div>) :
        <table className="games-table"><thead><tr>
          <th scope="col">#</th><th scope="col">{t('games.colName')}</th><th scope="col">{t('games.colProfile')}</th>
          <th scope="col"><span style={{ position: 'absolute', left: -9999 }}>{t('games.colActions')}</span></th></tr></thead>
          <tbody>{rules.map((rule, index) => {
            const known = nameOf(rule.ProfileID) !== undefined;
            const primary = rule.Name || rule.Executable;
            const secondary = rule.Folder || rule.Executable;
            return <tr key={`${rule.Folder}\n${rule.Executable}`}>
              <td className="n">{index + 1}</td>
              <td><strong className="games-name">{primary}</strong>
                {secondary !== primary && <span className="games-sub">{secondary}</span>}</td>
              <td><span className={`games-profile-select${known ? '' : ' invalid'}`}>
                <GnomeSelect aria-label={t('games.profileFor', { name: primary })} value={rule.ProfileID} placeholder={t('games.profileMissing')}
                  options={profileOptions} disabled={controlsDisabled}
                  onChange={value => void persist(enabled, rules.map((r, i) => i === index ? { ...r, ProfileID: value } : r))} />
              </span>{!known && loaded && <span className="help games-error">{t('games.profileGone', { name: primary })}</span>}</td>
              <td className="games-actions">
                <button type="button" className="order" aria-label={t('games.moveUp', { name: primary })} disabled={controlsDisabled || index === 0} onClick={() => move(index, -1)}>↑</button>{' '}
                <button type="button" className="order" aria-label={t('games.moveDown', { name: primary })} disabled={controlsDisabled || index === rules.length - 1} onClick={() => move(index, 1)}>↓</button>{' '}
                <button type="button" className="small danger" aria-label={t('games.removeLabel', { name: primary })} disabled={controlsDisabled}
                  onClick={() => void persist(enabled, rules.filter((_, i) => i !== index))}>{t('games.remove')}</button>
              </td></tr>;
          })}</tbody></table>}
      <div className="games-addbar">
        <button type="button" className="primary" disabled={controlsDisabled || !profiles.length} onClick={openDialog}>{t('games.add')}</button>
      </div>
      {adding && <AddGameDialog service={service} rules={rules} profileOptions={profileOptions} profile={selectedForm}
        error={error} onError={setError} onProfile={setFormProfile} onAdd={addRule} onClose={closeDialog} />}
    </section>
    <section className="games-card" aria-labelledby="games-launch-title">
      <div className="games-launch">
        <div className="games-launch-head">
          <div><strong id="games-launch-title">{t('games.launchTitle')}</strong>
            <span className="help">{t('games.launchHelp')}</span></div>
          <GnomeSelect aria-label={t('games.launchProfile')} value={selectedCommand} placeholder={t('games.noProfiles')}
            options={profileOptions} disabled={!profiles.length} onChange={value => { setCommandProfile(value); setCopy(undefined); }} />
        </div>
        <div className="games-cmd">
          <code ref={commandRef} aria-label={t('games.launchCommand')}>{command}</code>
          <button type="button" disabled={!profiles.length} onClick={() => void copyCommand()}>{t('games.copy')}</button>
        </div>
        <p className={`help games-status${copy?.ok ? ' ok' : ''}`} role="status">{copy?.text}</p>
        <ol className="games-steps"><li>{t('games.step1')}</li><li>{t('games.step2')}</li><li>{t('games.step3')}</li></ol>
      </div>
    </section>
    <p className="help games-error" role="alert" hidden={!error || adding}>{error}</p>
  </div>;
}
