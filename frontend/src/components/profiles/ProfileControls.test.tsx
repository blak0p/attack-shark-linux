import '@testing-library/jest-dom/vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ProfileControls } from './ProfileControls';
afterEach(cleanup);
const initial = { Profiles: [{ ID: 'a', Name: 'Automatic profile', CreatedAt: '2026-01-01' }, { ID: 'b', Name: 'Gaming', CreatedAt: '2026-01-02' }], ActiveID: 'a', Notice: 'Created automatically at 2026-01-01; no mouse changes.', Error: { Code: '' }, Applied: false };
function fixture() {
 const service = { GetProfiles: vi.fn().mockResolvedValue(initial), SwitchProfile: vi.fn().mockResolvedValue({ ...initial, Error: { Code: 'partial_failure' } }), CreateProfile: vi.fn().mockResolvedValue(initial), RenameProfile: vi.fn().mockResolvedValue(initial), DuplicateProfile: vi.fn().mockResolvedValue(initial), DeleteProfile: vi.fn().mockResolvedValue(initial), DismissProfileNotice: vi.fn().mockResolvedValue({ ...initial, Notice: '' }) };
 return { service, refresh: vi.fn().mockResolvedValue(undefined) };
}
it('shows notice, dismisses it, reports partial switches and refreshes actual settings without optimistic selection', async () => {
 const f = fixture(); render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await screen.findByText(initial.Notice);
 fireEvent.click(screen.getByRole('button', { name: 'Dismiss profile notice' }));
 await waitFor(() => expect(screen.queryByText(initial.Notice)).not.toBeInTheDocument());
 fireEvent.change(screen.getByLabelText('Local profile'), { target: { value: 'b' } });
 await screen.findByRole('alert'); expect(screen.getByRole('alert')).toHaveTextContent('partial_failure');
 expect(screen.getByLabelText('Local profile')).toHaveValue('a'); expect(f.refresh).toHaveBeenCalledOnce();
});
it('offers inline CRUD and explains protected deletion', async () => {
 const f = fixture(); render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 await screen.findByRole('option', { name: 'Gaming' });
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
 await screen.findByRole('option', { name: 'Gaming' });
 fireEvent.change(screen.getByLabelText('Local profile'), { target: { value: 'b' } });
 expect(screen.getByLabelText('Local profile')).toBeDisabled();
 expect(screen.getByRole('button', { name: 'Manage profiles' })).toBeDisabled();
 expect(screen.queryByText('Profile applied.')).not.toBeInTheDocument();
 finish({ ...initial, ActiveID: 'b', Applied: true });
 await screen.findByText('Profile applied.');
 expect(screen.getByLabelText('Local profile')).toHaveValue('b');
 f.service.SwitchProfile.mockRejectedValueOnce(new Error('disconnected'));
 fireEvent.change(screen.getByLabelText('Local profile'), { target: { value: 'a' } });
 await screen.findByText('Profile switch failed; mouse settings may be partial.');
 expect(f.refresh).toHaveBeenCalledTimes(2);
 expect(screen.getByLabelText('Local profile')).toHaveValue('b');
 expect(screen.queryByText('Profile applied.')).not.toBeInTheDocument();
});
it('disables profiles without a selected mouse and does not call the service', () => {
 const f = fixture(); render(<ProfileControls {...f} scope="none" disabled onBusy={() => {}} />);
 expect(screen.getByLabelText('Local profile')).toBeDisabled(); expect(f.service.GetProfiles).not.toHaveBeenCalled();
});
it('ignores obsolete results after mouse changes', async () => {
 const f = fixture(); let resolve!: (v: typeof initial) => void;
 f.service.GetProfiles.mockImplementationOnce(() => new Promise(r => { resolve = r; }));
 const view = render(<ProfileControls {...f} scope="alpha" disabled={false} onBusy={() => {}} />);
 view.rerender(<ProfileControls {...f} scope="beta" disabled={false} onBusy={() => {}} />);
 await screen.findByRole('option', { name: 'Gaming' });
 resolve({ ...initial, ActiveID: 'b' });
 await waitFor(() => expect(screen.getByLabelText('Local profile')).toHaveValue('a'));
});
