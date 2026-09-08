import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import type { LightingEffect } from "../../desktop-contract";

type LightingEffectSelectProps = {
  effects: LightingEffect[];
  value: number;
  disabled?: boolean;
  onChange: (effect: LightingEffect) => void;
};

export function LightingEffectSelect({ effects, value, disabled = false, onChange }: LightingEffectSelectProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const listboxId = useId();
  const selectedIndex = effects.findIndex((effect) => effect.Mode === value);
  const currentIndex = selectedIndex >= 0 ? selectedIndex : 0;
  const selectedEffect = effects[selectedIndex];
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(currentIndex);

  useEffect(() => {
    if (!open) setActiveIndex(currentIndex);
  }, [currentIndex, open]);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideMouseDown = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", closeOnOutsideMouseDown);
    return () => document.removeEventListener("mousedown", closeOnOutsideMouseDown);
  }, [open]);

  const chooseEffect = (effect: LightingEffect) => {
    onChange(effect);
    setOpen(false);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) return;
    if (event.key === "Escape") {
      if (open) {
        event.preventDefault();
        setOpen(false);
      }
      return;
    }
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (!open) {
        setOpen(true);
        setActiveIndex(currentIndex);
      } else if (effects[activeIndex]) {
        chooseEffect(effects[activeIndex]);
      }
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!open) {
        setOpen(true);
        setActiveIndex(currentIndex);
        return;
      }
      const direction = event.key === "ArrowDown" ? 1 : -1;
      setActiveIndex((index) => effects.length ? (index + direction + effects.length) % effects.length : 0);
      return;
    }
    if (open && (event.key === "Home" || event.key === "End")) {
      event.preventDefault();
      setActiveIndex(event.key === "Home" ? 0 : Math.max(0, effects.length - 1));
    }
  };

  return (
    <div className="lighting-effect-combobox" ref={containerRef}>
      <button
        type="button"
        className="lighting-effect-trigger"
        role="combobox"
        aria-label="Lighting effect"
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-controls={listboxId}
        aria-activedescendant={open && effects[activeIndex] ? `${listboxId}-option-${activeIndex}` : undefined}
        disabled={disabled}
        onClick={() => {
          setOpen((isOpen) => !isOpen);
          setActiveIndex(currentIndex);
        }}
        onKeyDown={handleKeyDown}
      >
        {selectedEffect?.Label ?? "Select effect"}
      </button>
      {open && (
        <div className="lighting-effect-listbox" id={listboxId} role="listbox" aria-label="Lighting effect">
          {effects.map((effect, index) => (
            <div
              className={`lighting-effect-option${index === activeIndex ? " is-active" : ""}`}
              id={`${listboxId}-option-${index}`}
              key={effect.Mode}
              role="option"
              aria-selected={index === selectedIndex}
              onClick={() => chooseEffect(effect)}
              onMouseEnter={() => setActiveIndex(index)}
            >
              {effect.Label}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
