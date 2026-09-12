import type { ReactNode } from "react";

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
  if (children) {
    return (
      <div className="group" role="group" aria-label="Mouse features">
        {children}
      </div>
    );
  }

  return (
    <div className="group" role="group" aria-label="Mouse features">
      <h2>Mouse features</h2>
      <div className="rows">
        <div className="row">
          <label htmlFor="angle-snap-toggle">
            <strong>Angle snap</strong>
            <span>Align sensor movement to straight lines</span>
          </label>
          <input
            id="angle-snap-toggle"
            className="switch"
            type="checkbox"
            aria-label="Angle snap"
            checked={angleSnap}
            disabled={disabled}
            onChange={(event) => onAngleSnapChange?.(event.target.checked)}
          />
        </div>
        <div className="row">
          <label htmlFor="ripple-control-toggle">
            <strong>Ripple control</strong>
            <span>Smooth sensor jitter at high DPI values</span>
          </label>
          <input
            id="ripple-control-toggle"
            className="switch"
            type="checkbox"
            aria-label="Ripple control"
            checked={rippleControl}
            disabled={disabled}
            onChange={(event) => onRippleControlChange?.(event.target.checked)}
          />
        </div>
        <div className="row">
          <label htmlFor="lod-select">
            <strong>Lift-off distance</strong>
            <span>Sensor tracking cutoff height</span>
          </label>
          <select
            id="lod-select"
            className="select"
            aria-label="Lift-off distance"
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
