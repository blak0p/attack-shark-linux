import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";

export type GnomeSelectOption = {
  value: string;
  label: string;
};

export type GnomeSelectProps = {
  id?: string;
  "aria-label": string;
  value: string;
  placeholder?: string;
  options: GnomeSelectOption[];
  disabled?: boolean;
  className?: string;
  onChange: (value: string) => void;
};

export function GnomeSelect({
  id,
  "aria-label": ariaLabel,
  value,
  placeholder,
  options,
  disabled = false,
  className = "",
  onChange,
}: GnomeSelectProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const listboxId = useId();
  const [open, setOpen] = useState(false);
  const selectedIndex = options.findIndex((opt) => opt.value === value);
  const currentIndex = selectedIndex >= 0 ? selectedIndex : 0;
  const [activeIndex, setActiveIndex] = useState(currentIndex);

  const selectedOption = selectedIndex >= 0 ? options[selectedIndex] : undefined;

  useEffect(() => {
    if (!open) {
      setActiveIndex(currentIndex);
    }
  }, [currentIndex, open]);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", closeOnOutsideClick);
    return () => document.removeEventListener("mousedown", closeOnOutsideClick);
  }, [open]);

  const handleSelect = (val: string) => {
    onChange(val);
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
      } else if (options[activeIndex]) {
        handleSelect(options[activeIndex].value);
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
      setActiveIndex((prev) =>
        options.length ? (prev + direction + options.length) % options.length : 0
      );
      return;
    }

    if (open && (event.key === "Home" || event.key === "End")) {
      event.preventDefault();
      setActiveIndex(event.key === "Home" ? 0 : Math.max(0, options.length - 1));
    }
  };

  return (
    <div className={`gnome-select-wrapper ${className}`} ref={containerRef}>
      <button
        id={id}
        type="button"
        className="select gnome-select-trigger"
        role="combobox"
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-controls={listboxId}
        aria-activedescendant={open && options[activeIndex] ? `${listboxId}-opt-${activeIndex}` : undefined}
        disabled={disabled}
        onClick={() => {
          if (!disabled) {
            setOpen((prev) => !prev);
            setActiveIndex(currentIndex);
          }
        }}
        onKeyDown={handleKeyDown}
      >
        <span className="gnome-select-value">
          {selectedOption?.label ?? placeholder ?? value}
        </span>
        <span className="gnome-select-arrow" aria-hidden="true">▾</span>
      </button>

      {open && (
        <div
          id={listboxId}
          className="gnome-select-popup"
          role="listbox"
          aria-label={ariaLabel}
        >
          {options.map((opt, index) => {
            const isSelected = opt.value === value;
            const isActive = index === activeIndex;
            return (
              <div
                id={`${listboxId}-opt-${index}`}
                key={opt.value}
                role="option"
                aria-selected={isSelected}
                className={`gnome-select-option ${isSelected ? "selected" : ""} ${isActive ? "active" : ""}`}
                onClick={() => handleSelect(opt.value)}
                onMouseEnter={() => setActiveIndex(index)}
              >
                {opt.label}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
