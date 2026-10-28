import '@testing-library/jest-dom/vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeAll, expect, it, vi } from 'vitest';
import type { InstalledGame, PickResult, WatcherRule, WatcherSettingsSnapshot } from '../../desktop-contract';
import { GamesPanel, launchCommand, shellQuote } from './GamesPanel';

beforeAll(() => {
  HTMLDialogElement.prototype.showModal = function () { this.open = true; };
  HTMLDialogElement.prototype.close = function () { this.open = false; };
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); });

const A = 'a'.repeat(32), B = 'b'.repeat(32), C = 'c'.repeat(32);
const profiles = {
  Profiles: [{ ID: A, Name: 'Automatic profile', CreatedAt: '' }, { ID: B, Name: 'FPS 800 DPI', CreatedAt: '' }, { ID: C, Name: 'macro', CreatedAt: '' }],
  ActiveID: B, Notice: '', Error: { Code: '' }, Applied: false,
};
const snap = (enabled: boolean, rules: WatcherRule[], code = ''): WatcherSettingsSnapshot =>
  ({ Enabled: enabled, Rules: rules, Error: { Code: code } });
const rules3: WatcherRule[] = [{ Executable: 'cs2', Folder: '', Name: '', ProfileID: B }, { Executable: 'Hades.exe', Folder: '', Name: '', ProfileID: C }, { Executable: 'eldenring.exe', Folder: '', Name: '', ProfileID: A }];
const folderRule: WatcherRule = { Executable: '', Folder: '/games/Terraria', Name: 'Terraria', ProfileID: C };
const catalog: InstalledGame[] = [
  { Name: 'Terraria', Source: 'Steam', Folder: '/steam/Terraria', Executable: '', IsGame: true },
  { Name: 'Rocket League', Source: 'Heroic', Folder: '/heroic/RL', Executable: '', IsGame: true },
  { Name: 'xemu', Source: 'App', Folder: '', Executable: 'xemu', IsGame: true },
  { Name: 'Firefox', Source: 'App', Folder: '', Executable: 'firefox', IsGame: false },
];
const noPick: PickResult = { Path: '', Name: '', Cancelled: true, Error: { Code: '' } };

function fixture(initial = snap(true, rules3)) {
  const service = {
    GetProfiles: vi.fn().mockResolvedValue(profiles),
    GetWatcherSettings: vi.fn().mockResolvedValue(initial),
    SetWatcherSettings: vi.fn(async (enabled: boolean, rules: WatcherRule[]) => snap(enabled, rules)),
    ListInstalledGames: vi.fn().mockResolvedValue({ Games: catalog, Error: { Code: '' } }),
    PickGameFolder: vi.fn().mockResolvedValue(noPick),
    PickGameExecutable: vi.fn().mockResolvedValue(noPick),
  };
  return service;
}
async function show(service = fixture()) {
  render(<GamesPanel service={service} />);
  await screen.findByRole('table');
  return service;
}
const lastSet = (s: ReturnType<typeof fixture>) => s.SetWatcherSettings.mock.calls.at(-1);
const exes = () => within(screen.getByRole('table')).getAllByRole('row').slice(1).map(r => r.querySelector('.games-name')!.textContent);

it('loads settings and profiles on mount and renders ordered rules with profile names', async () => {
  const s = await show();
  expect(s.GetWatcherSettings).toHaveBeenCalledOnce();
  expect(s.GetProfiles).toHaveBeenCalledOnce();
  expect(s.SetWatcherSettings).not.toHaveBeenCalled();
  expect(exes()).toEqual(['cs2', 'Hades.exe', 'eldenring.exe']);
  expect(screen.getByRole('checkbox', { name: 'Switch profile automatically' })).toBeChecked();
  expect(screen.getByRole('button', { name: 'Profile for cs2' })).toHaveTextContent('FPS 800 DPI');
  expect(screen.getByRole('button', { name: 'Profile for Hades.exe' })).toHaveTextContent('macro');
  for (const h of ['#', 'Name', 'Profile']) expect(screen.getByRole('columnheader', { name: h })).toHaveAttribute('scope', 'col');
  expect(screen.getByRole('button', { name: 'Move cs2 up' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Move eldenring.exe down' })).toBeDisabled();
  expect(screen.getByText('Works while this app is open.', { exact: false })).toBeVisible();
  expect(document.body.textContent).not.toMatch(/tray/i);
});

it('shows an empty state when there are no rules', async () => {
  render(<GamesPanel service={fixture(snap(false, []))} />);
  expect(await screen.findByText('No rules yet')).toBeVisible();
  expect(screen.queryByRole('table')).not.toBeInTheDocument();
});

it('persists the switch with the full rules list and renders the returned snapshot', async () => {
  const s = await show();
  fireEvent.click(screen.getByRole('checkbox', { name: 'Switch profile automatically' }));
  await waitFor(() => expect(screen.getByRole('checkbox', { name: 'Switch profile automatically' })).not.toBeChecked());
  expect(s.SetWatcherSettings).toHaveBeenCalledOnce();
  expect(lastSet(s)).toEqual([false, rules3]);
});

it('keeps rules editable while the switch is off and explains it with one helper line', async () => {
  const s = fixture(snap(false, rules3));
  render(<GamesPanel service={s} />);
  await screen.findByRole('table');
  const note = 'Automatic switching is off. Your rules are saved and will apply when you turn it on.';
  expect(screen.getByText(note)).toBeVisible();
  expect(screen.getByRole('region', { name: 'Rules' })).not.toHaveClass('off');
  for (const name of ['Move cs2 down', 'Remove cs2', 'Profile for cs2', 'Add a game']) expect(screen.getByRole('button', { name })).toBeEnabled();
  fireEvent.click(screen.getByRole('button', { name: 'Move cs2 down' }));
  await waitFor(() => expect(lastSet(s)).toEqual([false, [rules3[1], rules3[0], rules3[2]]]));
  fireEvent.click(screen.getByRole('checkbox', { name: 'Switch profile automatically' }));
  await waitFor(() => expect(screen.queryByText(note)).not.toBeInTheDocument());
});

it('does not show the off helper line while the switch is on', async () => {
  await show();
  expect(screen.queryByText(/Automatic switching is off/)).not.toBeInTheDocument();
});

it('sends the exact full list after reorder, remove, profile change and add', async () => {
  const s = await show();
  fireEvent.click(screen.getByRole('button', { name: 'Move Hades.exe up' }));
  await waitFor(() => expect(exes()).toEqual(['Hades.exe', 'cs2', 'eldenring.exe']));
  expect(lastSet(s)).toEqual([true, [rules3[1], rules3[0], rules3[2]]]);
  fireEvent.click(screen.getByRole('button', { name: 'Move Hades.exe down' }));
  await waitFor(() => expect(exes()).toEqual(['cs2', 'Hades.exe', 'eldenring.exe']));
  expect(lastSet(s)).toEqual([true, rules3]);
  fireEvent.click(screen.getByRole('button', { name: 'Remove Hades.exe' }));
  await waitFor(() => expect(exes()).toEqual(['cs2', 'eldenring.exe']));
  expect(lastSet(s)).toEqual([true, [rules3[0], rules3[2]]]);
  fireEvent.click(screen.getByRole('button', { name: 'Profile for cs2' }));
  fireEvent.click(screen.getByRole('menuitem', { name: 'macro', exact: true }));
  await waitFor(() => expect(lastSet(s)).toEqual([true, [{ ...rules3[0], ProfileID: C }, rules3[2]]]));
  expect(s.SetWatcherSettings).toHaveBeenCalledTimes(4);
});

async function openDialog(service = fixture()) {
  await show(service);
  fireEvent.click(screen.getByRole('button', { name: 'Add a game' }));
  const dialog = screen.getByRole('dialog', { name: 'Add a game' });
  await within(dialog).findByRole('textbox', { name: 'Search games' });
  return { service, dialog };
}
const itemNames = (dialog: HTMLElement) => within(dialog).queryAllByRole('button', { pressed: false }).map(b => b.querySelector('.games-item-name')?.textContent).filter(Boolean);

it('lists games first and other apps in a collapsed group, with source badges', async () => {
  const { dialog } = await openDialog();
  expect(within(dialog).getByRole('heading', { name: 'Games' })).toBeVisible();
  const others = dialog.querySelector('details.games-others') as HTMLDetailsElement;
  expect(others.open).toBe(false);
  expect(within(others).getByText('Other apps')).toBeVisible();
  expect(within(others).getByRole('button', { name: /Firefox/ })).toHaveTextContent('App');
  expect(within(dialog).getByRole('button', { name: /Terraria/ })).toHaveTextContent('Steam');
  expect(within(dialog).getByRole('button', { name: /Rocket League/ })).toHaveTextContent('Heroic');
  expect(within(dialog).getByRole('button', { name: /xemu/ })).toHaveTextContent('App');
  expect(others.contains(within(dialog).getByRole('button', { name: /xemu/ }))).toBe(false);
  expect(itemNames(dialog)).toEqual(['Terraria', 'Rocket League', 'xemu', 'Firefox']);
});

it('filters case-insensitively across both groups and opens the other apps when they match', async () => {
  const { dialog } = await openDialog();
  const search = within(dialog).getByRole('textbox', { name: 'Search games' });
  fireEvent.change(search, { target: { value: 'TERRA' } });
  expect(itemNames(dialog)).toEqual(['Terraria']);
  expect(dialog.querySelector('details.games-others')).toBeNull();
  fireEvent.change(search, { target: { value: 'fire' } });
  expect(itemNames(dialog)).toEqual(['Firefox']);
  expect((dialog.querySelector('details.games-others') as HTMLDetailsElement).open).toBe(true);
  fireEvent.change(search, { target: { value: 'zzz' } });
  expect(itemNames(dialog)).toEqual([]);
  expect(within(dialog).getByText('No games match your search.')).toBeVisible();
});

it('shows the empty state when no games are installed', async () => {
  const s = fixture(); s.ListInstalledGames.mockResolvedValue({ Games: [], Error: { Code: '' } });
  const { dialog } = await openDialog(s);
  expect(await within(dialog).findByText('No installed games found. Use Browse below.')).toBeVisible();
  expect(within(dialog).getByRole('button', { name: 'Browse for a game folder…' })).toBeEnabled();
});

async function addFromList(name: RegExp, profileName = 'FPS 800 DPI') {
  const ctx = await openDialog();
  fireEvent.click(await within(ctx.dialog).findByRole('button', { name }));
  expect(within(ctx.dialog).getByRole('button', { name: 'Profile' })).toHaveTextContent(profileName);
  fireEvent.click(within(ctx.dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(ctx.service.SetWatcherSettings).toHaveBeenCalledOnce());
  return ctx;
}

it.each([
  ['Steam item', /Terraria/, { Executable: '', Folder: '/steam/Terraria', Name: 'Terraria' }],
  ['Heroic item', /Rocket League/, { Executable: '', Folder: '/heroic/RL', Name: 'Rocket League' }],
  ['App item', /xemu/, { Executable: 'xemu', Folder: '', Name: 'xemu' }],
  ['other App item', /Firefox/, { Executable: 'firefox', Folder: '', Name: 'Firefox' }],
])('adds a %s with the exact full rules list, appended at the end', async (_kind, name, shape) => {
  const { service } = await addFromList(name);
  expect(lastSet(service)).toEqual([true, [...rules3, { ...shape, ProfileID: B }]]);
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  expect(exes().at(-1)).toBe(shape.Name);
});

it('uses the chosen profile for the new rule', async () => {
  const { service, dialog } = await openDialog();
  fireEvent.click(within(dialog).getByRole('button', { name: /Terraria/ }));
  fireEvent.click(within(dialog).getByRole('button', { name: 'Profile' }));
  fireEvent.click(screen.getByRole('menuitem', { name: 'Automatic profile', exact: true }));
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(lastSet(service)![1].at(-1)).toMatchObject({ Name: 'Terraria', ProfileID: A }));
});

it('adds a picked folder as a folder rule named by the result', async () => {
  const s = fixture(); s.PickGameFolder.mockResolvedValue({ Path: '/games/Hades', Name: 'Hades II', Cancelled: false, Error: { Code: '' } });
  const { dialog } = await openDialog(s);
  fireEvent.click(within(dialog).getByRole('button', { name: 'Browse for a game folder…' }));
  expect(await within(dialog).findByText('Hades II')).toBeVisible();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(lastSet(s)).toEqual([true, [...rules3, { Executable: '', Folder: '/games/Hades', Name: 'Hades II', ProfileID: B }]]));
});

it('adds a picked executable as an executable rule with its base name', async () => {
  const s = fixture(); s.PickGameExecutable.mockResolvedValue({ Path: '/opt/games/Celeste.bin', Name: 'Celeste', Cancelled: false, Error: { Code: '' } });
  const { dialog } = await openDialog(s);
  fireEvent.click(within(dialog).getByRole('button', { name: 'Browse for an executable…' }));
  expect(await within(dialog).findByText('Celeste')).toBeVisible();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(lastSet(s)).toEqual([true, [...rules3, { Executable: 'Celeste.bin', Folder: '', Name: 'Celeste', ProfileID: B }]]));
});

it('adds nothing when the file chooser is cancelled', async () => {
  const { service, dialog } = await openDialog();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Browse for a game folder…' }));
  await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Browse for an executable…' })).toBeEnabled());
  fireEvent.click(within(dialog).getByRole('button', { name: 'Browse for an executable…' }));
  await waitFor(() => expect(service.PickGameExecutable).toHaveBeenCalledOnce());
  expect(service.PickGameFolder).toHaveBeenCalledOnce();
  expect(within(dialog).queryByText(/^Selected:/)).not.toBeInTheDocument();
  expect(service.SetWatcherSettings).not.toHaveBeenCalled();
  expect(within(dialog).queryByRole('alert')).not.toBeInTheDocument();
});

it('adds a manual executable name with the text as display name', async () => {
  const { service, dialog } = await openDialog();
  fireEvent.click(within(dialog).getByText('Enter an executable name'));
  fireEvent.change(within(dialog).getByRole('textbox', { name: 'Executable name' }), { target: { value: '  Celeste.exe ' } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(lastSet(service)).toEqual([true, [...rules3, { Executable: 'Celeste.exe', Folder: '', Name: 'Celeste.exe', ProfileID: B }]]));
});

it.each([
  ['', 'Enter the executable name only, without a path.'],
  ['   ', 'Enter the executable name only, without a path.'],
  ['/usr/bin/cs2', 'Enter the executable name only, without a path.'],
  ['C:\\Games\\Hades.exe', 'Enter the executable name only, without a path.'],
  [' cs2 ', 'A rule for cs2 already exists.'],
])('rejects manual %j without calling the backend', async (value, message) => {
  const { service, dialog } = await openDialog();
  fireEvent.change(within(dialog).getByRole('textbox', { name: 'Executable name' }), { target: { value } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  expect(within(dialog).getByRole('alert')).toHaveTextContent(message);
  expect(service.SetWatcherSettings).not.toHaveBeenCalled();
  expect(exes()).toEqual(['cs2', 'Hades.exe', 'eldenring.exe']);
});

it('treats executable names as case-sensitive when checking duplicates', async () => {
  const { service, dialog } = await openDialog();
  fireEvent.change(within(dialog).getByRole('textbox', { name: 'Executable name' }), { target: { value: 'CS2' } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(service.SetWatcherSettings).toHaveBeenCalledOnce());
});

it('rejects a duplicate folder and a duplicate executable from the list without calling the backend', async () => {
  const s = fixture(snap(true, [...rules3, { ...folderRule, Folder: '/steam/Terraria' }]));
  const { dialog } = await openDialog(s);
  fireEvent.click(within(dialog).getByRole('button', { name: /Terraria/ }));
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  expect(within(dialog).getByRole('alert')).toHaveTextContent('A rule for Terraria already exists.');
  fireEvent.click(within(dialog).getByRole('button', { name: /Firefox/ }));
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(s.SetWatcherSettings).toHaveBeenCalledOnce());
  s.SetWatcherSettings.mockClear();
  fireEvent.click(screen.getByRole('button', { name: 'Add a game' }));
  const again = screen.getByRole('dialog', { name: 'Add a game' });
  fireEvent.click(await within(again).findByRole('button', { name: /Firefox/ }));
  fireEvent.click(within(again).getByRole('button', { name: 'Add' }));
  expect(within(again).getByRole('alert')).toHaveTextContent('A rule for Firefox already exists.');
  expect(s.SetWatcherSettings).not.toHaveBeenCalled();
});

it('rejects a picked folder that is already a rule', async () => {
  const s = fixture(snap(true, [folderRule]));
  s.PickGameFolder.mockResolvedValue({ Path: '/games/Terraria', Name: 'Terraria again', Cancelled: false, Error: { Code: '' } });
  const { dialog } = await openDialog(s);
  fireEvent.click(within(dialog).getByRole('button', { name: 'Browse for a game folder…' }));
  await within(dialog).findByText('Terraria again');
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  expect(within(dialog).getByRole('alert')).toHaveTextContent('already exists');
  expect(s.SetWatcherSettings).not.toHaveBeenCalled();
});

it.each([
  ['PickGameFolder', 'Browse for a game folder…'],
  ['PickGameExecutable', 'Browse for an executable…'],
] as const)('shows pick_failed from %s', async (method, label) => {
  const s = fixture(); s[method].mockResolvedValue({ ...noPick, Cancelled: false, Error: { Code: 'pick_failed' } });
  const { dialog } = await openDialog(s);
  fireEvent.click(within(dialog).getByRole('button', { name: label }));
  expect(await within(dialog).findByRole('alert')).toHaveTextContent('The file chooser could not be opened.');
  expect(s.SetWatcherSettings).not.toHaveBeenCalled();
});

it('shows games_unavailable when the catalog cannot be read', async () => {
  const s = fixture(); s.ListInstalledGames.mockResolvedValue({ Games: [], Error: { Code: 'games_unavailable' } });
  const { dialog } = await openDialog(s);
  expect(await within(dialog).findByRole('alert')).toHaveTextContent('The list of installed games is not available.');
});

it('falls back to the manual input when the games methods are absent', async () => {
  const { GetProfiles, GetWatcherSettings, SetWatcherSettings } = fixture();
  const s = { GetProfiles, GetWatcherSettings, SetWatcherSettings };
  render(<GamesPanel service={s} />);
  await screen.findByRole('table');
  fireEvent.click(screen.getByRole('button', { name: 'Add a game' }));
  const dialog = screen.getByRole('dialog', { name: 'Add a game' });
  expect(within(dialog).queryByRole('textbox', { name: 'Search games' })).not.toBeInTheDocument();
  expect(within(dialog).queryByRole('button', { name: /Browse/ })).not.toBeInTheDocument();
  fireEvent.change(within(dialog).getByRole('textbox', { name: 'Executable name' }), { target: { value: 'Celeste.exe' } });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Add' }));
  await waitFor(() => expect(SetWatcherSettings).toHaveBeenCalledWith(true, [...rules3, { Executable: 'Celeste.exe', Folder: '', Name: 'Celeste.exe', ProfileID: B }]));
});

it('keeps Folder and Name of every rule when reordering or changing the profile', async () => {
  const list = [folderRule, rules3[0]];
  const s = fixture(snap(true, list));
  await show(s);
  expect(screen.getByText('/games/Terraria')).toBeVisible();
  expect(screen.getByRole('button', { name: 'Profile for Terraria' })).toHaveTextContent('macro');
  fireEvent.click(screen.getByRole('button', { name: 'Move Terraria down' }));
  await waitFor(() => expect(lastSet(s)).toEqual([true, [list[1], folderRule]]));
  fireEvent.click(screen.getByRole('button', { name: 'Profile for Terraria' }));
  fireEvent.click(screen.getByRole('menuitem', { name: 'Automatic profile', exact: true }));
  await waitFor(() => expect(lastSet(s)).toEqual([true, [list[1], { ...folderRule, ProfileID: A }]]));
  fireEvent.click(screen.getByRole('button', { name: 'Remove cs2' }));
  await waitFor(() => expect(lastSet(s)).toEqual([true, [{ ...folderRule, ProfileID: A }]]));
});

it('shows the display name as primary text and the executable as the muted line', async () => {
  await show(fixture(snap(true, [{ Executable: 'xemu', Folder: '', Name: 'Xemu', ProfileID: A }, rules3[0]])));
  const row = screen.getByRole('button', { name: 'Profile for Xemu' }).closest('tr')!;
  expect(row.querySelector('.games-name')).toHaveTextContent('Xemu');
  expect(row.querySelector('.games-sub')).toHaveTextContent('xemu');
  expect(screen.getByRole('button', { name: 'Remove cs2' }).closest('tr')!.querySelector('.games-sub')).toBeNull();
});

it.each([
  ['invalid_watcher_settings', 'That rule is not valid. Use the executable name only, without a path.'],
  ['watcher_unavailable', 'Automatic switching is not available.'],
])('shows the stored state and %s as an alert', async (code, message) => {
  const s = fixture();
  s.SetWatcherSettings.mockResolvedValueOnce(snap(true, rules3, code));
  await show(s);
  fireEvent.click(screen.getByRole('button', { name: 'Remove cs2' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(message);
  expect(exes()).toEqual(['cs2', 'Hades.exe', 'eldenring.exe']);
});

it('flags a rule whose profile no longer exists', async () => {
  render(<GamesPanel service={fixture(snap(true, [{ ...rules3[0], ProfileID: 'f'.repeat(32) }]))} />);
  const select = await screen.findByRole('button', { name: 'Profile for cs2' });
  expect(select).toHaveTextContent('Profile missing');
  expect(select.closest('.games-profile-select')).toHaveClass('invalid');
  expect(screen.getByText('The profile for cs2 no longer exists. Choose another profile.')).toBeVisible();
});

it.each([
  ['FPS 800 DPI', 'FPS 800 DPI'],
  ['my game', 'my game'],
  ['say "hi"', 'say \\"hi\\"'],
  ['cost $5', 'cost \\$5'],
  ['run `x`', 'run \\`x\\`'],
  ['back\\slash', 'back\\\\slash'],
])('shell-quotes %j', (name, quoted) => {
  expect(shellQuote(name)).toBe(quoted);
  expect(launchCommand(name)).toBe(`attack-shark-linux --profile "${quoted}" -- %command%`);
});

it('shows the launch command for the active profile and follows the selected profile', async () => {
  await show();
  expect(screen.getByLabelText('Launch command')).toHaveTextContent('attack-shark-linux --profile "FPS 800 DPI" -- %command%');
  fireEvent.click(screen.getByRole('button', { name: 'Profile for the launch command' }));
  fireEvent.click(screen.getByRole('menuitem', { name: 'macro', exact: true }));
  expect(screen.getByLabelText('Launch command')).toHaveTextContent('attack-shark-linux --profile "macro" -- %command%');
  expect(screen.getAllByRole('listitem')).toHaveLength(3);
  expect(screen.getByText('In Steam, right-click the game and open Properties.')).toBeVisible();
});

it('copies the command and reports success', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined);
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
  await show();
  fireEvent.click(screen.getByRole('button', { name: 'Copy' }));
  expect(await screen.findByText('Copied to the clipboard.')).toBeVisible();
  expect(writeText).toHaveBeenCalledWith('attack-shark-linux --profile "FPS 800 DPI" -- %command%');
});

it('falls back to a manual copy hint when the clipboard is unavailable', async () => {
  Object.defineProperty(navigator, 'clipboard', { value: { writeText: vi.fn().mockRejectedValue(new Error('denied')) }, configurable: true });
  await show();
  fireEvent.click(screen.getByRole('button', { name: 'Copy' }));
  expect(await screen.findByText('Press Ctrl+C to copy the selected command.')).toBeVisible();
  expect(screen.queryByText('Copied to the clipboard.')).not.toBeInTheDocument();
});
