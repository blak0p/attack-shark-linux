import { useEffect, useId, useRef, useState } from 'react';
import type { GamesService, InstalledGame, PickResult, WatcherRule } from '../../desktop-contract';
import { GnomeSelect } from '../panels/GnomeSelect';

export const gamesErrors: Record<string, string> = {
  pick_failed: 'The file chooser could not be opened.',
  games_unavailable: 'The list of installed games is not available.',
};

type Choice = { Name: string; Folder: string; Executable: string };
type ProfileOption = { value: string; label: string; direct: boolean };

const isGame = (game: InstalledGame) => game.Source !== 'App' || game.IsGame;
const choiceFrom = (game: InstalledGame): Choice => game.Source === 'App'
  ? { Name: game.Name, Executable: game.Executable, Folder: '' }
  : { Name: game.Name, Folder: game.Folder, Executable: '' };
const byName = (a: InstalledGame, b: InstalledGame) => {
  const x = a.Name.toLowerCase(), y = b.Name.toLowerCase();
  return x < y ? -1 : x > y ? 1 : 0;
};
const sources = [['All', 'All'], ['Steam', 'Steam'], ['Heroic', 'Heroic'], ['App', 'Apps']] as const;
type SourceFilter = typeof sources[number][0];

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
  const [source, setSource] = useState<SourceFilter>('All');
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
  const matching = (games ?? []).filter(game => (source === 'All' || game.Source === source) && game.Name.toLowerCase().includes(needle));
  const visible = [...matching.filter(isGame).sort(byName), ...matching.filter(game => !isGame(game)).sort(byName)];

  const pick = async () => {
    setPicking(true); onError('');
    try {
      const result: PickResult | undefined = await service.PickGameFolder?.();
      if (!result || result.Cancelled) return;
      if (result.Error.Code) { onError(gamesErrors[result.Error.Code] ?? result.Error.Code); return; }
      setChoice({ Name: result.Name, Folder: result.Path, Executable: '' });
    } catch { onError(gamesErrors.pick_failed); } finally { setPicking(false); }
  };

  const submit = async (event: { preventDefault(): void }) => {
    event.preventDefault();
    if (adding || !profile) return;
    if (!choice) return;
    const next = choice;
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
        onClick={() => { setChoice(own); onError(''); }}>
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
        <div className="games-dialog-top">
          <input type="text" aria-label="Search games" placeholder="Search games" value={query}
            onChange={event => setQuery(event.target.value)} />
          <div role="group" aria-label="Filter by source" className="games-chips">
            {sources.map(([value, label]) => <button key={value} type="button" className="games-chip"
              aria-pressed={source === value} onClick={() => setSource(value)}>{label}</button>)}
          </div>
        </div>
        <div className="games-list">
          {games && (games.length === 0
            ? <p className="help games-list-empty">No installed games found. Use Browse below.</p>
            : visible.length === 0 && <p className="help games-list-empty">Nothing matches your search.</p>)}
          {visible.length > 0 && <ul className="games-items">{visible.map(item)}</ul>}
        </div>
      </>}
      <div className="games-dialog-bottom">
        {service.PickGameFolder && <div className="games-browse">
          <button type="button" disabled={picking} onClick={() => void pick()}>Browse for a game folder…</button>
        </div>}
        {choice && <p className="help games-selected">Selected: <strong>{choice.Name}</strong>{' '}
          <span>{choice.Folder || choice.Executable}</span></p>}
      <label className="games-dialog-profile"><span>Profile</span>
        <GnomeSelect aria-label="Profile" value={profile} placeholder="No profiles" options={profileOptions}
          disabled={!profileOptions.length} portalTarget={() => dialogRef.current} onChange={onProfile} />
      </label>
      <p className="help games-error" role="alert" hidden={!error}>{error}</p>
      <div className="games-dialog-footer">
        <button type="button" onClick={onClose}>Cancel</button>
        <button type="submit" className="primary" disabled={!profileOptions.length || adding || !choice}>Add</button>
      </div>
      </div>
    </form>
  </dialog>;
}
