import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";

export type GnomeSelectOption = {
  value: string;
  label: string;
  group?: string;
  disabled?: boolean;
};

type IndexedOption = GnomeSelectOption & { index: number };
type OptionGroup = { name?: string; options: IndexedOption[] };

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
  const groupedOptions = options.reduce<OptionGroup[]>((groups, option, index) => {
    const group = groups.at(-1);
    if (group?.name === option.group) group.options.push({ ...option, index });
    else groups.push({ name: option.group, options: [{ ...option, index }] });
    return groups;
  }, []);

  const nextEnabledIndex = (start: number, direction: number) => {
    for (let step = 1; step <= options.length; step++) {
      const index = (start + direction * step + options.length) % options.length;
      if (!options[index].disabled) return index;
    }
    return start;
  };

  useEffect(() => {
    if (!open) setActiveIndex(currentIndex);
  }, [currentIndex, open]);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!containerRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", closeOnOutsideClick);
    return () => document.removeEventListener("mousedown", closeOnOutsideClick);
  }, [open]);

  const handleSelect = (option: GnomeSelectOption) => {
    if (option.disabled) return;
    onChange(option.value);
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
      } else if (options[activeIndex]) handleSelect(options[activeIndex]);
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (!open) {
        setOpen(true);
        setActiveIndex(currentIndex);
        return;
      }
      setActiveIndex((previous) => nextEnabledIndex(previous, event.key === "ArrowDown" ? 1 : -1));
      return;
    }
    if (open && (event.key === "Home" || event.key === "End")) {
      event.preventDefault();
      const start = event.key === "Home" ? -1 : 0;
      const direction = event.key === "Home" ? 1 : -1;
      setActiveIndex(nextEnabledIndex(start, direction));
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
            setOpen((previous) => !previous);
            setActiveIndex(currentIndex);
          }
        }}
        onKeyDown={handleKeyDown}
      >
        <span className="gnome-select-value">{selectedOption?.label ?? placeholder ?? value}</span>
        <span className="gnome-select-arrow" aria-hidden="true">▾</span>
      </button>

      {open && (
        <div id={listboxId} className="gnome-select-popup" role="listbox" aria-label={ariaLabel}>
          {groupedOptions.map((group) => (
            group.name ? (
              <div key={group.name} role="group" aria-label={group.name}>
                <div className="gnome-select-group-label">{group.name}</div>
                {group.options.map((option) => <Option key={option.value} option={option} />)}
              </div>
            ) : group.options.map((option) => <Option key={option.value} option={option} />)
          ))}
        </div>
      )}
    </div>
  );

  function Option({ option }: { option: IndexedOption }) {
    const isSelected = option.value === value;
    const isActive = option.index === activeIndex;
    return (
      <div
        id={`${listboxId}-opt-${option.index}`}
        role="option"
        aria-selected={isSelected}
        aria-disabled={option.disabled ?? false}
        className={`gnome-select-option ${isSelected ? "selected" : ""} ${isActive ? "active" : ""} ${option.disabled ? "disabled" : ""}`}
        onClick={() => handleSelect(option)}
        onMouseEnter={() => !option.disabled && setActiveIndex(option.index)}
      >
        {option.label}
      </div>
    );
  }
}
