import '@testing-library/jest-dom/vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ProfileControls } from './ProfileControls';
afterEach(() => { cleanup(); vi.useRealTimers(); });
it('expires informational notices without dismissing stored data and cleans timers on scope change', async () => {
 const f = fixture(); const view = render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await screen.findByText(initial.Notice);
 vi.useFakeTimers();
 // Reload a different scope so the new notice starts its timer under the fake clock.
 view.rerender(<ProfileControls {...f} scope="beta" disabled={false} onBusy={() => {}} />);
 await act(async () => {});
 expect(screen.getByText(initial.Notice)).toBeVisible();
 act(() => vi.advanceTimersByTime(8000));
 expect(screen.queryByText(initial.Notice)).not.toBeInTheDocument();
 view.rerender(<ProfileControls {...f} scope="beta" disabled={false} onBusy={() => {}} />);
 expect(screen.queryByText(initial.Notice)).not.toBeInTheDocument();
 expect(f.service.DismissProfileNotice).not.toHaveBeenCalled();
 view.unmount(); expect(vi.getTimerCount()).toBe(0);
});
const initial = { Profiles: [{ ID: 'a', Name: 'Automatic profile', CreatedAt: '2026-01-01' }, { ID: 'b', Name: 'Gaming', CreatedAt: '2026-01-02' }], ActiveID: 'a', Notice: 'Created automatically at 2026-01-01; no mouse changes.', Error: { Code: '' }, Applied: false };
function fixture() {
 const service = { GetProfiles: vi.fn().mockResolvedValue(initial), SwitchProfile: vi.fn().mockResolvedValue({ ...initial, Error: { Code: 'partial_failure' } }), CreateProfile: vi.fn().mockResolvedValue(initial), RenameProfile: vi.fn().mockResolvedValue(initial), DuplicateProfile: vi.fn().mockResolvedValue(initial), DeleteProfile: vi.fn().mockResolvedValue(initial), DismissProfileNotice: vi.fn().mockResolvedValue({ ...initial, Notice: '' }) };
 return { service, refresh: vi.fn().mockResolvedValue(undefined) };
}
it('expires switch errors after eight seconds and successes after three without changing apply behavior', async () => {
 const f = fixture(); render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await waitFor(() => expect(screen.getByLabelText('Profile')).toBeEnabled());
 vi.useFakeTimers();
 fireEvent.click(screen.getByLabelText('Profile'));
 fireEvent.click(screen.getByRole('menuitem', { name: 'Gaming', exact: true }));
 await act(async () => {});
 expect(screen.getByRole('alert')).toHaveTextContent('partial_failure');
 act(() => vi.advanceTimersByTime(8000));
 expect(screen.queryByRole('alert')).not.toBeInTheDocument();
 f.service.SwitchProfile.mockResolvedValueOnce({ ...initial, ActiveID: 'b', Applied: true, Error: { Code: '' } });
 fireEvent.click(screen.getByLabelText('Profile'));
 fireEvent.click(screen.getByRole('menuitem', { name: 'Gaming', exact: true }));
 await act(async () => {});
 expect(screen.getByText('Profile applied.')).toBeVisible();
 act(() => vi.advanceTimersByTime(3000));
 expect(screen.queryByText('Profile applied.')).not.toBeInTheDocument();
 expect(f.service.SwitchProfile).toHaveBeenCalledTimes(2);
 expect(f.refresh).toHaveBeenCalledTimes(2);
});
it('shows notice, dismisses it, reports partial switches and refreshes actual settings without optimistic selection', async () => {
 const f = fixture(); render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await screen.findByText(initial.Notice);
 fireEvent.click(screen.getByRole('button', { name: 'Dismiss profile notice' }));
 await waitFor(() => expect(screen.queryByText(initial.Notice)).not.toBeInTheDocument());
 fireEvent.click(screen.getByRole('button', { name: 'Profile', exact: true }));
 fireEvent.click(screen.getByRole('menuitem', { name: 'Gaming', exact: true }));
 await screen.findByRole('alert'); expect(screen.getByRole('alert')).toHaveTextContent('partial_failure');
 expect(screen.getByLabelText('Profile')).toHaveTextContent('Automatic profile'); expect(f.refresh).toHaveBeenCalledOnce();
});
it('presents a named panel with compact actions and a close control', async () => {
 const f = fixture(); render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await waitFor(() => expect(screen.getByRole('button', { name: 'Profile', exact: true })).toBeEnabled());
 fireEvent.click(screen.getByRole('button', { name: 'Manage profiles' }));
 expect(screen.getByRole('button', { name: 'Manage profiles' })).toHaveAttribute('title', 'Manage profiles');
 expect(screen.getByRole('button', { name: 'Manage profiles' })).not.toHaveTextContent('Manage profiles');
 expect(screen.getByRole('region', { name: 'Profile management' })).toBeVisible();
 expect(screen.getByRole('button', { name: 'Rename Gaming' })).toHaveTextContent(/^Rename$/);
 expect(screen.getByText('Active')).toBeVisible();
 fireEvent.click(screen.getByRole('button', { name: 'Close profile management' }));
 expect(screen.queryByRole('region', { name: 'Profile management' })).not.toBeInTheDocument();
 expect(f.service.CreateProfile).not.toHaveBeenCalled();
});
it('offers inline CRUD and explains protected deletion', async () => {
 const f = fixture(); render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await waitFor(() => expect(screen.getByRole('button', { name: 'Profile', exact: true })).toBeEnabled());
 fireEvent.click(screen.getByRole('button', { name: 'Manage profiles' }));
 expect(screen.getByRole('button', { name: 'Delete Automatic profile' })).toBeDisabled();
 for (const operation of ['Create', 'Rename', 'Duplicate']) {
  fireEvent.click(screen.getByRole('button', { name: operation === 'Create' ? 'Create profile' : `${operation} Automatic profile` }));
  fireEvent.change(screen.getByLabelText('Profile name'), { target: { value: 'Work' } });
  fireEvent.click(screen.getByRole('button', { name: 'Confirm profile action' }));
  await waitFor(() => expect(f.service[`${operation}Profile` as 'CreateProfile']).toHaveBeenCalled());
 }
 fireEvent.click(screen.getByRole('button', { name: 'Delete Gaming' }));
 fireEvent.click(screen.getByRole('button', { name: 'Confirm profile action' }));
 await waitFor(() => expect(f.service.DeleteProfile).toHaveBeenCalledWith('b'));
});
it('only reports acknowledged switches, prevents concurrent actions and permits retry after rejection', async () => {
 const f = fixture(); let finish!: (v: typeof initial) => void;
 f.service.SwitchProfile.mockImplementationOnce(() => new Promise(r => { finish = r; }));
 render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await waitFor(() => expect(screen.getByRole('button', { name: 'Profile', exact: true })).toBeEnabled());
 fireEvent.click(screen.getByRole('button', { name: 'Profile', exact: true }));
 fireEvent.click(screen.getByRole('menuitem', { name: 'Gaming', exact: true }));
 expect(screen.getByLabelText('Profile')).toBeDisabled();
 expect(screen.getByRole('button', { name: 'Manage profiles' })).toBeDisabled();
 expect(screen.queryByText('Profile applied.')).not.toBeInTheDocument();
 finish({ ...initial, ActiveID: 'b', Applied: true });
 await screen.findByText('Profile applied.');
 expect(screen.getByLabelText('Profile')).toHaveTextContent('Gaming');
 f.service.SwitchProfile.mockRejectedValueOnce(new Error('disconnected'));
 fireEvent.click(screen.getByRole('button', { name: 'Profile', exact: true }));
 fireEvent.click(screen.getByRole('menuitem', { name: 'Automatic profile', exact: true }));
 await screen.findByText('Profile switch failed; mouse settings may be partial.');
 expect(f.refresh).toHaveBeenCalledTimes(2);
 expect(screen.getByLabelText('Profile')).toHaveTextContent('Gaming');
 expect(screen.queryByText('Profile applied.')).not.toBeInTheDocument();
});
it('disables profiles without a selected mouse and does not call the service', () => {
 const f = fixture(); render(<ProfileControls {...f} scope="none" disabled onBusy={() => {}} />);
 expect(screen.getByLabelText('Profile')).toBeDisabled(); expect(f.service.GetProfiles).not.toHaveBeenCalled();
});
it('ignores obsolete results after mouse changes', async () => {
 const f = fixture(); let resolve!: (v: typeof initial) => void;
 f.service.GetProfiles.mockImplementationOnce(() => new Promise(r => { resolve = r; }));
 const view = render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 view.rerender(<ProfileControls {...f} scope="beta" disabled={false} onBusy={() => {}} />);
 await waitFor(() => expect(screen.getByRole('button', { name: 'Profile', exact: true })).toBeEnabled());
 resolve({ ...initial, ActiveID: 'b' });
 await waitFor(() => expect(screen.getByLabelText('Profile')).toHaveTextContent('Automatic profile'));
});

it('uses the themed profile menu with direct choices and keyboard dismissal', async () => {
 const f = fixture(); render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 const trigger = screen.getByRole('button', { name: 'Profile', exact: true });
 await waitFor(() => expect(trigger).toBeEnabled());
 fireEvent.keyDown(trigger, { key: 'ArrowDown' });
 expect(screen.getByRole('menuitem', { name: 'Gaming', exact: true })).toBeVisible();
 fireEvent.keyDown(trigger, { key: 'Escape' });
 expect(screen.queryByRole('menuitem', { name: 'Gaming', exact: true })).not.toBeInTheDocument();
 expect(f.service.SwitchProfile).not.toHaveBeenCalled();
});
