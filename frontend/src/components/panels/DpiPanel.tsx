import type { CSSProperties, ReactNode } from "react";
import { useTranslation } from "react-i18next";

export type DpiStage = {
  index: number;
  dpi: number;
};

export type DpiPanelProps = {
  stages?: DpiStage[];
  activeIndex?: number;
  activeDPI?: number | null;
  activeColor?: number[] | null;
  colorFor?: (index: number) => string | null;
  ready?: boolean;
  onSelectStage?: (index: number) => void;
  onStageDPI?: (index: number, dpi: number) => void;
  children?: ReactNode;
};

const DPI_MIN = 50;
const DPI_MAX = 26000;
const DPI_STEP = 50;

const position = (value: number) =>
  Math.round(Math.max(0, Math.min(1, (value - DPI_MIN) / (DPI_MAX - DPI_MIN))) * 1000) / 10;

export function DpiPanel({
  stages,
  activeIndex = 0,
  activeDPI,
  activeColor,
  colorFor,
  ready = true,
  onSelectStage,
  onStageDPI,
  children,
}: DpiPanelProps) {
  const { t } = useTranslation();
  if (children) {
    return (
      <section id="dpi" className="card panel" aria-labelledby="dpi-title">
        {children}
      </section>
    );
  }

  const displayStages = (stages ?? []).slice(0, 6);
  const activeStage = displayStages.find((s) => s.index === activeIndex) ?? displayStages[0];

  return (
    <article id="dpi" className="card" aria-labelledby="dpi-title">
      <div className="card-head">
        <div>
          <h2 id="dpi-title">{t("dpi.title")}</h2>
          <p className="hint">
            {activeIndex >= 0 ? (
              activeDPI != null ? (
                <>
                  {t("dpi.active", { dpi: activeDPI })}
                  {activeColor && (
                    <span
                      className="color-swatch"
                      style={{ background: `rgb(${activeColor.join(", ")})`, marginLeft: 6 }}
                    />
                  )}
                </>
              ) : (
                t("dpi.unknown")
              )
            ) : (
              t("dpi.none")
            )}
          </p>
        </div>
        <div className="value">
          {activeDPI != null ? activeDPI : "—"} <small>DPI</small>
        </div>
      </div>

      <div className="stages" role="group" aria-label={t("dpi.activeStage")}>
        {displayStages.map(({ index, dpi }) => {
          const isSelected = index === activeIndex;
          const color = colorFor ? colorFor(index) : null;
          return (
            <button
              key={index}
              type="button"
              className={`stage${isSelected ? " selected active" : ""}`}
              aria-pressed={isSelected}
              aria-label={isSelected ? t("dpi.stageActive", { n: index + 1 }) : t("dpi.stage", { n: index + 1 })}
              disabled={!ready}
              onClick={() => onSelectStage?.(index)}
              style={{ "--dot-color": color ?? "#3b82f6" } as CSSProperties}
            >
              {t("dpi.stage", { n: index + 1 })}
              <b>{dpi}</b>
            </button>
          );
        })}
      </div>

      {activeStage && (
        <div className="range-row">
          <span>{DPI_MIN}</span>
          <input
            type="range"
            className="stage-slider"
            aria-label={t("dpi.stageDpi", { n: activeStage.index + 1 })}
            min={DPI_MIN}
            max={DPI_MAX}
            step={DPI_STEP}
            value={activeStage.dpi}
            disabled={!ready}
            onChange={(event) => onStageDPI?.(activeStage.index, Number(event.target.value))}
            style={
              {
                "--fill": `${position(activeStage.dpi)}%`,
                "--stage-color": colorFor ? colorFor(activeStage.index) ?? "#3b82f6" : "#3b82f6",
              } as CSSProperties
            }
          />
          <span>26,000</span>
        </div>
      )}
    </article>
  );
}
