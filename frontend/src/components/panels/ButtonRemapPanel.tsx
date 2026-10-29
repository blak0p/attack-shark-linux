import { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useTranslation } from "react-i18next";
import { i18n } from "../../i18n";
import type { Macro, MacroDraft, MacroPlaybackMode, MacroProgress, RemapAction as Action } from "../../desktop-contract";
import { deviceAdmission } from "../../macros/device-admission";
import { GnomeSelect, type GnomeSelectOption } from "./GnomeSelect";

type Button = { Button: number; Action: Action | null; PreservedDefault: string };
type Remap = {
  Pending: { Buttons: Button[] };
  Applied: { Buttons: Button[] };
  Actions: Action[];
  Firmware: string;
  Error: { Code: string };
  MacroPending?: MacroDraft | null;
  MacroApplied?: MacroDraft | null;
  MacroDrafts?: Record<number, MacroDraft>;
  MacroAppliedDrafts?: Record<number, MacroDraft>;
  MacroProgress?: MacroProgress;
};

const compatible = (macro: Macro) => deviceAdmission(macro.events).eligible;

const labelFor = (action: Action) => i18n.t(`remap.actions.${action}`);

const isMultimedia = (action: Action) => ["media_player", "play_pause", "stop", "previous_track", "next_track", "volume_up", "volume_down", "mute"].includes(action);
const isMouseControls = (action: Action) => ["scroll_up", "scroll_down", "dpi_cycle", "dpi_plus", "dpi_minus"].includes(action);
const isBrowser = (action: Action) => ["browser_calculator", "browser_email", "browser_forward", "browser_backward", "browser_stop", "browser_my_computer", "browser_refresh", "browser_home", "browser_search"].includes(action);

const getButtonMacroDraft = (remap: Remap, buttonNum: number): MacroDraft | null => {
  if (remap.MacroDrafts !== undefined) {
    return remap.MacroDrafts[buttonNum] ?? null;
  }
  if (remap.MacroPending && remap.MacroPending.Button === buttonNum) {
    return remap.MacroPending;
  }
  return null;
};

const getAllStagedDrafts = (remap: Remap): MacroDraft[] => {
  if (remap.MacroDrafts && Object.keys(remap.MacroDrafts).length > 0) {
    return Object.values(remap.MacroDrafts);
  }
  if (remap.MacroPending) {
    return [remap.MacroPending];
  }
  return [];
};

const getAllAppliedDrafts = (remap: Remap): MacroDraft[] => {
  if (remap.MacroAppliedDrafts && Object.keys(remap.MacroAppliedDrafts).length > 0) {
    return Object.values(remap.MacroAppliedDrafts);
  }
  if (remap.MacroApplied) {
    return [remap.MacroApplied];
  }
  return [];
};

const normalizeDrafts = (draftsMap?: Record<number, MacroDraft>, single?: MacroDraft | null) => {
  if (draftsMap && Object.keys(draftsMap).length > 0) {
    const keys = Object.keys(draftsMap).map(Number).sort((a, b) => a - b);
    return keys.map((k) => ({ ...draftsMap[k], Mode: draftsMap[k].Mode ?? 0 }));
  }
  if (single) return [{ ...single, Mode: single.Mode ?? 0 }];
  return [];
};

const playbackKeys = ["remap.playback.times", "remap.playback.toggle", "remap.playback.hold"] as const;

function MacroAssignmentDialog({ button, initialID, initialRepeat, initialMode, macros, available, onCancel, onConfirm }: {
  button: number;
  initialID: string;
  initialRepeat: number;
  initialMode: MacroPlaybackMode;
  macros: Macro[];
  available: boolean;
  onCancel(): void;
  onConfirm(id: string, repeat: number, mode: MacroPlaybackMode): void;
}) {
  const { t } = useTranslation();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const titleID = useId();
  const [id, setID] = useState(initialID);
  const [repeat, setRepeat] = useState(String(initialRepeat));
  const [mode, setMode] = useState(initialMode);
  const count = mode === 0 ? Number(repeat) : 1;
  const selected = macros.find((macro) => macro.id === id);
  const valid = available && !!selected && (mode !== 0 || repeat.trim() !== "") &&
    deviceAdmission(selected.events, count, mode).eligible;

  useEffect(() => {
    const opener = document.activeElement as HTMLElement | null;
    const dialog = dialogRef.current!;
    dialog.showModal();
    return () => {
      dialog.close();
      if (opener?.isConnected) opener.focus();
    };
  }, []);

  return (
    <dialog ref={dialogRef} className="macro-assignment-dialog card" aria-labelledby={titleID}
      onCancel={(event) => { event.preventDefault(); onCancel(); }}
      onKeyDown={(event) => {
        event.stopPropagation();
        if (event.key === "Escape") { event.preventDefault(); onCancel(); }
      }}>
      <form onSubmit={(event) => { event.preventDefault(); if (valid) onConfirm(id, count, mode); }}>
        <h2 id={titleID}>{t("remap.dialog.title", { button })}</h2>
        <label>
          <span>{t("remap.dialog.savedMacro")}</span>
          <GnomeSelect aria-label={t("remap.dialog.savedMacro")} value={id} disabled={!available || !macros.length}
            portalTarget={() => dialogRef.current}
            placeholder={id && !macros.some((macro) => macro.id === id) ? t("remap.dialog.versionUnavailable")
              : macros.length ? t("remap.dialog.choose") : t("remap.dialog.noneCompatible")}
            options={macros.map((macro) => ({ value: macro.id, label: macro.name, direct: true }))}
            onChange={setID} />
        </label>
        <label>
          <span>{t("remap.dialog.playbackMode")}</span>
          <GnomeSelect aria-label={t("remap.dialog.playbackMode")} value={String(mode)} disabled={!available}
            portalTarget={() => dialogRef.current}
            options={playbackKeys.map((key, value) => ({ value: String(value), label: t(key), direct: true }))}
            onChange={(value) => setMode(Number(value) as MacroPlaybackMode)} />
        </label>
        {mode === 1 && <p className="hint">{t("remap.dialog.intendedToggle")}</p>}
        {mode === 2 && <p className="hint">{t("remap.dialog.intendedHold")}</p>}
        {mode === 0 && <label>
          <span>{t("remap.dialog.repeat")}</span>
          <input className="input" aria-label={t("remap.dialog.repeat")} type="number" min={1} max={255} step={1}
            value={repeat} disabled={!available} onChange={(event) => setRepeat(event.target.value)} />
        </label>}
        <div className="actions">
          <button className="button" type="button" onClick={onCancel}>{t("common.cancel")}</button>
          <button className="button primary" type="submit" disabled={!valid}>{t("common.confirm")}</button>
        </div>
      </form>
    </dialog>
  );
}

export function ButtonRemapPanel({
  feedbackManaged = false,
  remap,
  ready,
  onStage,
  onApply = () => {},
  onDiscard = () => {},
  feedbackFor,
  macros = [],
  libraryReady = true,
  onStageMacro,
  onClearMacro,
  error = "",
  assignmentIntent: _assignmentIntent,
  assignmentScope,
  assignmentAvailable = ready,
  onConsumeIntent: _onConsumeIntent,
}: {
  feedbackManaged?: boolean;
  remap: Remap;
  ready: boolean;
  macros?: Macro[];
  libraryReady?: boolean;
  onStageMacro?(id: string, button: number, repeat: number, mode?: MacroPlaybackMode): void;
  onClearMacro?(button: number): void;
  error?: string;
  onStage(button: number, action: Action): void;
  onApply?(): void;
  onDiscard?(): void;
  feedbackFor?: (code: string) => string;
  assignmentIntent?: { token: number; id: string };
  assignmentScope?: unknown;
  assignmentAvailable?: boolean;
  onConsumeIntent?(token: number): void;
}) {
  const { t } = useTranslation();
  const isAvailable = ready && assignmentAvailable;
  const [macroDialog, setMacroDialog] = useState<{ button: number; id: string; repeat: number; mode: MacroPlaybackMode; scope: unknown } | null>(null);

  useEffect(() => {
    setMacroDialog(null);
  }, [assignmentScope]);

  const assignmentLabel = (button: Button) => {
    const draft = getButtonMacroDraft(remap, button.Button);
    if (draft) {
      return draft.Mode === 1 || draft.Mode === 2
        ? `${draft.Name} — ${t(playbackKeys[draft.Mode])}` : `${draft.Name} × ${draft.Repeat}`;
    }
    return button.Action ? labelFor(button.Action) : button.PreservedDefault || t("remap.default");
  };

  const compatibleMacros = libraryReady && isAvailable ? macros.filter(compatible) : [];

  const optionsForButton = (button: Button) => {
    const actionOptions: GnomeSelectOption[] = remap.Actions.map((action) => ({
      value: action,
      label: labelFor(action),
      group: isBrowser(action) ? t("remap.groups.browser") : isMouseControls(action) ? t("remap.groups.mouse") : isMultimedia(action) ? t("remap.groups.multimedia") : t("remap.groups.basic"),
      disabled: button.Button === 1 && (isMultimedia(action) || isMouseControls(action)),
    }));

    return [...actionOptions, {
      value: "macro", label: t("remap.macro"), direct: true,
      disabled: !libraryReady || !onStageMacro,
    }];
  };

  const stagedDrafts = getAllStagedDrafts(remap);
  const appliedDrafts = getAllAppliedDrafts(remap);
  const macroTransport = stagedDrafts.length > 0 || appliedDrafts.length > 0;
  const draftMatchesApplied = JSON.stringify(normalizeDrafts(remap.MacroDrafts, remap.MacroPending)) ===
    JSON.stringify(normalizeDrafts(remap.MacroAppliedDrafts, remap.MacroApplied)) &&
    JSON.stringify(remap.Pending) === JSON.stringify(remap.Applied);

  const residualMacroProgress = !macroTransport && !!(remap.MacroProgress?.Assignment || remap.MacroProgress?.Upload);

  return (
    <article
      id="remapping-card"
      className="card"
      aria-labelledby="button-remap-title"
      tabIndex={0}
      onKeyDown={(event) => {
        if (!isAvailable || event.target !== event.currentTarget) return;
        if (event.key === "Enter") {
          event.preventDefault();
          onApply();
        }
        if (event.key === "Escape") {
          event.preventDefault();
          onDiscard();
        }
      }}
    >
      <h2 id="button-remap-title">{t("nav.remapping")}</h2>

      {!libraryReady && <p className="hint">{t("remap.libraryUnavailable")}</p>}
      {libraryReady && macros.some(macro => !compatible(macro)) && <p className="hint">{t("remap.someUnsendable")}</p>}
      {!feedbackManaged && error && <p role="alert">{error}</p>}

      {remap.Pending.Buttons.map((button) => {
        const draft = getButtonMacroDraft(remap, button.Button);
        const assignedLabel = assignmentLabel(button);
        const selectValue = draft ? "" : (button.Action ?? "");

        return (
          <div className="binding remap-binding" key={button.Button}>
            <div>
              <b>
                {t("remap.button", { n: button.Button })}
                {button.PreservedDefault ? ` (${button.PreservedDefault})` : ""}
              </b>
              <span>
                {assignedLabel}
              </span>
            </div>
            <div className="remap-button-controls">
              <GnomeSelect
                aria-label={t("remap.buttonAction", { n: button.Button })}
                disabled={!isAvailable}
                value={selectValue}
                placeholder={assignedLabel}
                options={optionsForButton(button)}
                onChange={(val) => {
                    if (val === "macro") {
                      if (isAvailable && libraryReady && onStageMacro) {
                        setMacroDialog({ button: button.Button, id: draft?.ID ?? "", repeat: draft?.Repeat ?? 1, mode: draft?.Mode ?? 0, scope: assignmentScope });
                      }
                      return;
                    }
                    const action = val as Action;
                    if (!(button.Button === 1 && (isMultimedia(action) || isMouseControls(action)))) {
                      onStage(button.Button, action);
                      if (draft) {
                        onClearMacro?.(button.Button);
                      }
                    }
                }}
              />
            </div>
          </div>
        );
      })}

      {macroDialog && macroDialog.scope === assignmentScope && createPortal(
        <MacroAssignmentDialog
          button={macroDialog.button}
          initialID={macroDialog.id}
          initialRepeat={macroDialog.repeat}
          initialMode={macroDialog.mode}
          macros={compatibleMacros}
          available={isAvailable && libraryReady && !!onStageMacro}
          onCancel={() => setMacroDialog(null)}
          onConfirm={(id, repeat, mode) => {
            if (!isAvailable || !libraryReady || !compatibleMacros.some((macro) => macro.id === id)) return;
            if (mode === 0) onStageMacro?.(id, macroDialog.button, repeat);
            else onStageMacro?.(id, macroDialog.button, repeat, mode);
            setMacroDialog(null);
          }}
        />,
        document.body,
      )}

      <details className="group"><summary>{t("remap.summary")}</summary>
      <p aria-label={t("remap.summaryLabel")} className="hint" style={{ marginTop: 14 }}>
        {remap.Pending.Buttons.map(
          (button) =>
            t("remap.summaryItem", { n: button.Button, label: assignmentLabel(button) })
        ).join(", ")}
      </p>
      </details>

      <div className="actions">
        <button type="button" className="button" disabled={!isAvailable} onClick={onDiscard}>
          {t("remap.discard")}
        </button>
        <button type="button" className="button primary" disabled={!isAvailable} onClick={onApply}>
          {t("remap.apply")}
        </button>
      </div>

      {residualMacroProgress && <p className="hint" title={t("remap.residualTitle")}>{t("feedback.devicePartial")}</p>}
      {(!feedbackManaged || macroTransport) && <details className="group" open={!feedbackManaged}><summary>{t("remap.deviceDetails")}</summary>
      <div className="status" role="status" aria-label={t("remap.status.label")}>
        {macroTransport
          ? remap.Firmware === "success"
            ? draftMatchesApplied
              ? t("remap.status.confirmed")
              : appliedDrafts.length > 0
                ? t("remap.status.lastConfirmed")
                : t("remap.status.stagedUnconfirmed")
            : remap.Firmware === "failed"
            ? t("remap.status.transportFailed")
            : t("remap.status.stagedNoTransport")
          : remap.Firmware === "success"
          ? t("remap.status.applied")
          : remap.Firmware === "failed"
          ? <>
              {t("remap.status.failed")}
              {remap.Error.Code && feedbackFor && <>: <span role="alert">{feedbackFor(remap.Error.Code)}</span></>}
            </>
          : t("remap.status.pending")}
      </div>
      </details>}
    </article>
  );
}
