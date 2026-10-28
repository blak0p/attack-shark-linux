import { useEffect, useId, useRef, useState } from 'react';
import type { GamesService, InstalledGame, PickResult, WatcherRule } from '../../desktop-contract';
import { GnomeSelect } from '../panels/GnomeSelect';

export const gamesErrors: Record<string, string> = {
  pick_failed: 'The file chooser could not be opened.',
  games_unavailable: 'The list of installed games is not available.',
};
export const pathError = 'Enter the executable name only, without a path.';

type Choice = { Name: string; Folder: string; Executable: string };
type ProfileOption = { value: string; label: string; direct: boolean };

const isGame = (game: InstalledGame) => game.Source !== 'App' || game.IsGame;
const choiceFrom = (game: InstalledGame): Choice => game.Source === 'App'
  ? { Name: game.Name, Executable: game.Executable, Folder: '' }
  : { Name: game.Name, Folder: game.Folder, Executable: '' };
const baseName = (path: string) => path.split(/[\\/]/).filter(Boolean).pop() ?? path;

export function AddGameDialog({ service, rules, profileOptions, profile, error, onError, onProfile, onAdd, onClose }: {
  service: Partial<GamesService>;
  rules: WatcherRule[];
  profileOptions: ProfileOption[];
  profile: string;
  error: string;
  onError(message: string): void;
  onProfile(id: string): void;
  onAdd(rule: WatcherRule): Promise<boolean>;
  onClose(): void;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleID = useId();
  const hasCatalog = !!service.ListInstalledGames;
  const [games, setGames] = useState<InstalledGame[]>();
  const [query, setQuery] = useState('');
  const [choice, setChoice] = useState<Choice>();
  const [manual, setManual] = useState('');
  const [manualOpen, setManualOpen] = useState(!hasCatalog);
  const [othersOpen, setOthersOpen] = useState(false);
  const [picking, setPicking] = useState(false);
  const [adding, setAdding] = useState(false);

  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    const dialog = dialogRef.current!;
    dialog.showModal();
    return () => { dialog.close(); if (opener?.isConnected) opener.focus(); };
  }, []);

  useEffect(() => {
    let current = true;
    void service.ListInstalledGames?.().then(next => {
      if (!current) return;
      setGames(next.Games ?? []);
      if (next.Error.Code) onError(gamesErrors[next.Error.Code] ?? next.Error.Code);
    }).catch(() => { if (current) { setGames([]); onError(gamesErrors.games_unavailable); } });
    return () => { current = false; };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [service]);

  const needle = query.trim().toLowerCase();
  const matching = (games ?? []).filter(game => game.Name.toLowerCase().includes(needle));
  const gameItems = matching.filter(isGame);
  const otherItems = matching.filter(game => !isGame(game));
  const forcedOpen = needle !== '' && otherItems.length > 0;

  const pick = async (method: 'PickGameFolder' | 'PickGameExecutable') => {
    setPicking(true); onError('');
    try {
      const result: PickResult | undefined = await service[method]?.();
      if (!result || result.Cancelled) return;
      if (result.Error.Code) { onError(gamesErrors[result.Error.Code] ?? result.Error.Code); return; }
      setManual('');
      setChoice(method === 'PickGameFolder'
        ? { Name: result.Name, Folder: result.Path, Executable: '' }
        : { Name: result.Name, Executable: baseName(result.Path), Folder: '' });
    } catch { onError(gamesErrors.pick_failed); } finally { setPicking(false); }
  };

  const submit = async (event: { preventDefault(): void }) => {
    event.preventDefault();
    if (adding || !profile) return;
    let next: Choice;
    if (choice) next = choice;
    else {
      const name = manual.trim();
      if (!name || /[\\/]/.test(name)) { setManualOpen(true); onError(pathError); return; }
      next = { Name: name, Executable: name, Folder: '' };
    }
    const duplicate = rules.some(rule => (next.Folder !== '' && rule.Folder === next.Folder) ||
      (next.Executable !== '' && rule.Executable === next.Executable));
    if (duplicate) { onError(`A rule for ${next.Name} already exists.`); return; }
    setAdding(true);
    try { await onAdd({ Executable: next.Executable, Folder: next.Folder, Name: next.Name, ProfileID: profile }); }
    finally { setAdding(false); }
  };

  const item = (game: InstalledGame) => {
    const own = choiceFrom(game);
    const selected = choice?.Name === own.Name && choice.Folder === own.Folder && choice.Executable === own.Executable;
    return <li key={`${game.Source}\n${game.Name}\n${game.Folder}\n${game.Executable}`}>
      <button type="button" className="games-item" aria-pressed={selected}
        onClick={() => { setChoice(own); setManual(''); onError(''); }}>
        <span className="games-item-name">{game.Name}</span>
        <span className="games-badge">{game.Source}</span>
      </button>
    </li>;
  };

  return <dialog ref={dialogRef} className="games-dialog card" aria-labelledby={titleID}
    onCancel={event => { event.preventDefault(); onClose(); }}
    onKeyDown={event => { event.stopPropagation(); if (event.key === 'Escape') { event.preventDefault(); onClose(); } }}>
    <form className="games-dialog-form" onSubmit={event => void submit(event)}>
      <h2 id={titleID}>Add a game</h2>
      {hasCatalog && <>
        <input type="text" aria-label="Search games" placeholder="Search games" value={query}
          onChange={event => setQuery(event.target.value)} />
        <div className="games-list">
          {games && (games.length === 0
            ? <p className="help games-list-empty">No installed games found. Use Browse below.</p>
            : matching.length === 0 && <p className="help games-list-empty">No games match your search.</p>)}
          {gameItems.length > 0 && <>
            <h3 className="games-group">Games</h3>
            <ul className="games-items">{gameItems.map(item)}</ul>
          </>}
          {otherItems.length > 0 && <details className="games-others" open={forcedOpen || othersOpen}
            onToggle={event => setOthersOpen(event.currentTarget.open)}>
            <summary>Other apps</summary>
            <ul className="games-items">{otherItems.map(item)}</ul>
          </details>}
        </div>
        <div className="games-browse">
          {service.PickGameFolder && <button type="button" disabled={picking} onClick={() => void pick('PickGameFolder')}>Browse for a game folder…</button>}
          {service.PickGameExecutable && <button type="button" disabled={picking} onClick={() => void pick('PickGameExecutable')}>Browse for an executable…</button>}
        </div>
      </>}
      {choice && <p className="help games-selected">Selected: <strong>{choice.Name}</strong>{' '}
        <span>{choice.Folder || choice.Executable}</span></p>}
      <details className="games-manual" open={manualOpen} onToggle={event => setManualOpen(event.currentTarget.open)}>
        <summary>Enter an executable name</summary>
        <input type="text" aria-label="Executable name" placeholder="Executable name, e.g. Hades.exe" value={manual}
          onChange={event => { setManual(event.target.value); setChoice(undefined); }} />
      </details>
      <label className="games-dialog-profile"><span>Profile</span>
        <GnomeSelect aria-label="Profile" value={profile} placeholder="No profiles" options={profileOptions}
          disabled={!profileOptions.length} portalTarget={() => dialogRef.current} onChange={onProfile} />
      </label>
      <p className="help games-error" role="alert" hidden={!error}>{error}</p>
      <div className="games-dialog-footer">
        <button type="button" onClick={onClose}>Cancel</button>
        <button type="submit" className="primary" disabled={!profileOptions.length || adding}>Add</button>
      </div>
    </form>
  </dialog>;
}
