import '@testing-library/jest-dom/vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { setLanguage } from '../../i18n';
import { GamesPanel, launchCommand } from './GamesPanel';
import { gamesErrors } from './AddGameDialog';

const A = 'a'.repeat(32);
const profiles = { Profiles: [{ ID: A, Name: 'FPS', CreatedAt: '' }], ActiveID: A, Notice: '', Error: { Code: '' }, Applied: false };
const rule = { Executable: 'cs2', Folder: '', Name: '', ProfileID: 'missing' };

describe('Spanish rendering of games', () => {
  const show = HTMLDialogElement.prototype.showModal, close = HTMLDialogElement.prototype.close;
  beforeAll(() => {
    HTMLDialogElement.prototype.showModal = function () { this.open = true; };
    HTMLDialogElement.prototype.close = function () { this.open = false; };
  });
  afterAll(() => { HTMLDialogElement.prototype.showModal = show; HTMLDialogElement.prototype.close = close; });
  beforeEach(async () => { window.localStorage.clear(); await setLanguage('es'); });
  afterEach(async () => { cleanup(); await setLanguage('en'); });

  const service = (code = '') => ({
    GetProfiles: vi.fn().mockResolvedValue(profiles),
    GetWatcherSettings: vi.fn().mockResolvedValue({ Enabled: false, Rules: [rule], Tray: false, Error: { Code: code } }),
    SetTrayEnabled: vi.fn(), SetAutostartEnabled: vi.fn(),
    ListInstalledGames: vi.fn().mockResolvedValue({ Games: [], Error: { Code: '' } }),
    PickGameFolder: vi.fn(),
  });

  it('translates cards, table, launch section and the watcher error', async () => {
    render(<GamesPanel service={service('watcher_unavailable')} />);
    await screen.findByRole('table');
    expect(screen.getByRole('checkbox', { name: 'Cambiar de perfil automáticamente' })).toBeInTheDocument();
    expect(screen.getByRole('checkbox', { name: 'Seguir ejecutándose en la bandeja' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Reglas' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Nombre' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Subir cs2' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Quitar cs2' })).toHaveTextContent('Quitar');
    expect(screen.getByText('El perfil de cs2 ya no existe. Elegí otro perfil.')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toHaveTextContent('El cambio automático no está disponible.');
    expect(screen.getByRole('button', { name: 'Copiar' })).toBeInTheDocument();
    expect(screen.getByText('Sin la app abierta')).toBeInTheDocument();
    expect(screen.getByLabelText('Comando de inicio')).toHaveTextContent(launchCommand('FPS'));
  });

  it('translates the add dialog, filters and error lookups', async () => {
    render(<GamesPanel service={service()} />);
    await screen.findByRole('table');
    fireEvent.click(screen.getByRole('button', { name: 'Agregar un juego' }));
    expect(await screen.findByText('No se encontraron juegos instalados. Usá Examinar abajo.')).toBeInTheDocument();
    expect(screen.getByRole('group', { name: 'Filtrar por origen' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Todos' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Apps' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Steam' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Examinar carpeta de un juego…' })).toBeInTheDocument();
    expect(gamesErrors.pick_failed).toBe('No se pudo abrir el selector de archivos.');
    expect(gamesErrors.unknown_code).toBeUndefined();
  });
});
