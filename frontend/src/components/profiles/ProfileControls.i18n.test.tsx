import '@testing-library/jest-dom/vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguage } from '../../i18n';
import { ProfileControls } from './ProfileControls';

const snapshot = { Profiles: [{ ID: 'a', Name: 'Automatic profile', CreatedAt: '' }, { ID: 'b', Name: 'Gaming', CreatedAt: '' }], ActiveID: 'a', Notice: '', Error: { Code: '' }, Applied: false };

describe('Spanish rendering of profile controls', () => {
  beforeEach(async () => { window.localStorage.clear(); await setLanguage('es'); });
  afterEach(async () => { cleanup(); await setLanguage('en'); });

  const setup = async (service: Record<string, unknown> = {}) => {
    const full = { GetProfiles: vi.fn().mockResolvedValue(snapshot), ...service };
    render(<ProfileControls service={full} scope="s" disabled={false} refresh={async () => {}} onBusy={() => {}} />);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Administrar perfiles' })).toBeEnabled());
  };

  it('translates the selector, management panel and dialogs', async () => {
    await setup();
    expect(screen.getByRole('region', { name: 'Perfiles locales' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Administrar perfiles' }));
    expect(screen.getByRole('region', { name: 'Administración de perfiles' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Perfiles del mouse' })).toBeInTheDocument();
    expect(screen.getByText('Activo')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Renombrar Gaming' })).toHaveTextContent('Renombrar');
    expect(screen.getByRole('button', { name: 'Duplicar Gaming' })).toHaveTextContent('Duplicar');
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar Gaming' }));
    expect(screen.getByRole('dialog', { name: 'Eliminar perfil' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancelar acción del perfil' })).toHaveTextContent('Cancelar');
    expect(screen.getByRole('button', { name: 'Confirmar acción del perfil' })).toHaveTextContent('Eliminar perfil');
  });

  it('translates the duplicate-name error and the generic failure', async () => {
    const duplicate = { ...snapshot, Error: { Code: 'duplicate_profile_name' } };
    await setup({ CreateProfile: vi.fn().mockResolvedValue(duplicate), DeleteProfile: vi.fn().mockRejectedValue(new Error('x')) });
    fireEvent.click(screen.getByRole('button', { name: 'Administrar perfiles' }));
    fireEvent.click(screen.getByRole('button', { name: 'Crear perfil' }));
    expect(screen.getByRole('dialog', { name: 'Crear perfil' })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Nombre del perfil'), { target: { value: 'Gaming' } });
    fireEvent.click(screen.getByRole('button', { name: 'Confirmar acción del perfil' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Ya existe un perfil con ese nombre. Elegí otro nombre.');
  });
});
