import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

export type MouseFeaturesPanelProps = {
  angleSnap?: boolean;
  rippleControl?: boolean;
  liftDistance?: number;
  disabled?: boolean;
  onAngleSnapChange?: (enabled: boolean) => void;
  onRippleControlChange?: (enabled: boolean) => void;
  onLiftDistanceChange?: (distance: number) => void;
  children?: ReactNode;
};

export function MouseFeaturesPanel({
  angleSnap = false,
  rippleControl = false,
  liftDistance = 1,
  disabled = false,
  onAngleSnapChange,
  onRippleControlChange,
  onLiftDistanceChange,
  children,
}: MouseFeaturesPanelProps) {
  const { t } = useTranslation();
  if (children) {
    return (
      <div className="group" role="group" aria-label={t("features.title")}>
        {children}
      </div>
    );
  }

  return (
    <div className="group" role="group" aria-label={t("features.title")}>
      <h2>{t("features.title")}</h2>
      <div className="rows">
        <div className="row">
          <label htmlFor="angle-snap-toggle">
            <strong>{t("features.angleSnap")}</strong>
            <span>{t("features.angleSnapHint")}</span>
          </label>
          <input
            id="angle-snap-toggle"
            className="switch"
            type="checkbox"
            aria-label={t("features.angleSnap")}
            checked={angleSnap}
            disabled={disabled}
            onChange={(event) => onAngleSnapChange?.(event.target.checked)}
          />
        </div>
        <div className="row">
          <label htmlFor="ripple-control-toggle">
            <strong>{t("features.ripple")}</strong>
            <span>{t("features.rippleHint")}</span>
          </label>
          <input
            id="ripple-control-toggle"
            className="switch"
            type="checkbox"
            aria-label={t("features.ripple")}
            checked={rippleControl}
            disabled={disabled}
            onChange={(event) => onRippleControlChange?.(event.target.checked)}
          />
        </div>
        <div className="row">
          <label htmlFor="lod-select">
            <strong>{t("features.lod")}</strong>
            <span>{t("features.lodHint")}</span>
          </label>
          <select
            id="lod-select"
            className="select"
            aria-label={t("features.lod")}
            value={String(liftDistance)}
            disabled={disabled}
            onChange={(event) => onLiftDistanceChange?.(Number(event.target.value))}
          >
            <option value="1">1 mm</option>
            <option value="0">2 mm</option>
          </select>
        </div>
      </div>
    </div>
  );
}
