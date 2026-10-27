import '@testing-library/jest-dom/vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import type { WatcherSettingsSnapshot } from '../../desktop-contract';
import { GamesPanel, launchCommand, shellQuote } from './GamesPanel';

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

const A = 'a'.repeat(32), B = 'b'.repeat(32), C = 'c'.repeat(32);
const profiles = {
  Profiles: [{ ID: A, Name: 'Automatic profile', CreatedAt: '' }, { ID: B, Name: 'FPS 800 DPI', CreatedAt: '' }, { ID: C, Name: 'macro', CreatedAt: '' }],
  ActiveID: B, Notice: '', Error: { Code: '' }, Applied: false,
};
const snap = (enabled: boolean, rules: { Executable: string; ProfileID: string }[], code = ''): WatcherSettingsSnapshot =>
  ({ Enabled: enabled, Rules: rules, Error: { Code: code } });
const rules3 = [{ Executable: 'cs2', ProfileID: B }, { Executable: 'Hades.exe', ProfileID: C }, { Executable: 'eldenring.exe', ProfileID: A }];

function fixture(initial = snap(true, rules3)) {
  const service = {
    GetProfiles: vi.fn().mockResolvedValue(profiles),
    GetWatcherSettings: vi.fn().mockResolvedValue(initial),
    SetWatcherSettings: vi.fn(async (enabled: boolean, rules: { Executable: string; ProfileID: string }[]) => snap(enabled, rules)),
  };
  return service;
}
async function show(service = fixture()) {
  render(<GamesPanel service={service} />);
  await screen.findByRole('table');
  return service;
}
const lastSet = (s: ReturnType<typeof fixture>) => s.SetWatcherSettings.mock.calls.at(-1);
const exes = () => within(screen.getByRole('table')).getAllByRole('row').slice(1).map(r => within(r).getByText(/./, { selector: 'code' }).textContent);

it('loads settings and profiles on mount and renders ordered rules with profile names', async () => {
  const s = await show();
  expect(s.GetWatcherSettings).toHaveBeenCalledOnce();
  expect(s.GetProfiles).toHaveBeenCalledOnce();
  expect(s.SetWatcherSettings).not.toHaveBeenCalled();
  expect(exes()).toEqual(['cs2', 'Hades.exe', 'eldenring.exe']);
  expect(screen.getByRole('checkbox', { name: 'Switch profile automatically' })).toBeChecked();
  expect(screen.getByRole('button', { name: 'Profile for cs2' })).toHaveTextContent('FPS 800 DPI');
  expect(screen.getByRole('button', { name: 'Profile for Hades.exe' })).toHaveTextContent('macro');
  for (const h of ['#', 'Executable', 'Profile']) expect(screen.getByRole('columnheader', { name: h })).toHaveAttribute('scope', 'col');
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

it('disables and dims rules and the form while the switch is off', async () => {
  render(<GamesPanel service={fixture(snap(false, rules3))} />);
  await screen.findByRole('table');
  expect(screen.getByRole('checkbox', { name: 'Switch profile automatically' })).not.toBeChecked();
  expect(screen.getByRole('region', { name: 'Rules' })).toHaveClass('off');
  for (const name of ['Move cs2 down', 'Remove cs2', 'Profile for cs2', 'Executable name', 'Profile', 'Add rule']) expect(screen.getByRole(name === 'Move cs2 down' || name === 'Remove cs2' || name === 'Add rule' || name === 'Profile for cs2' || name === 'Profile' ? 'button' : 'textbox', { name })).toBeDisabled();
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
  await waitFor(() => expect(lastSet(s)).toEqual([true, [{ Executable: 'cs2', ProfileID: C }, rules3[2]]]));
  fireEvent.change(screen.getByRole('textbox', { name: 'Executable name' }), { target: { value: '  Hades.exe ' } });
  fireEvent.click(screen.getByRole('button', { name: 'Profile', exact: true }));
  fireEvent.click(screen.getByRole('menuitem', { name: 'Automatic profile', exact: true }));
  fireEvent.click(screen.getByRole('button', { name: 'Add rule' }));
  await waitFor(() => expect(exes()).toEqual(['cs2', 'eldenring.exe', 'Hades.exe']));
  expect(lastSet(s)).toEqual([true, [{ Executable: 'cs2', ProfileID: C }, rules3[2], { Executable: 'Hades.exe', ProfileID: A }]]);
  expect(s.SetWatcherSettings).toHaveBeenCalledTimes(5);
  expect(screen.getByRole('textbox', { name: 'Executable name' })).toHaveValue('');
});

it.each([
  ['', 'Enter the executable name only, without a path.'],
  ['   ', 'Enter the executable name only, without a path.'],
  ['/usr/bin/cs2', 'Enter the executable name only, without a path.'],
  ['C:\\Games\\Hades.exe', 'Enter the executable name only, without a path.'],
  [' cs2 ', 'A rule for cs2 already exists.'],
])('rejects %j without calling the backend', async (value, message) => {
  const s = await show();
  fireEvent.change(screen.getByRole('textbox', { name: 'Executable name' }), { target: { value } });
  fireEvent.click(screen.getByRole('button', { name: 'Add rule' }));
  expect(screen.getByRole('alert')).toHaveTextContent(message);
  expect(s.SetWatcherSettings).not.toHaveBeenCalled();
  expect(exes()).toEqual(['cs2', 'Hades.exe', 'eldenring.exe']);
});

it('treats executable names as case-sensitive when checking duplicates', async () => {
  const s = await show();
  fireEvent.change(screen.getByRole('textbox', { name: 'Executable name' }), { target: { value: 'CS2' } });
  fireEvent.click(screen.getByRole('button', { name: 'Add rule' }));
  await waitFor(() => expect(s.SetWatcherSettings).toHaveBeenCalledOnce());
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
  render(<GamesPanel service={fixture(snap(true, [{ Executable: 'cs2', ProfileID: 'f'.repeat(32) }]))} />);
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
