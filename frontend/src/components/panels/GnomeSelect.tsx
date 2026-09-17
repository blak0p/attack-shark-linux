import { useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";

export type GnomeSelectOption = {
  value: string;
  label: string;
  group?: string;
  disabled?: boolean;
};

type Category = { name: string; options: GnomeSelectOption[] };

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
  const triggerRef = useRef<HTMLButtonElement>(null);
  const categoryMenuRef = useRef<HTMLDivElement>(null);
  const menuId = useId();
  const categories = options.reduce<Category[]>((result, option) => {
    const name = option.group ?? "Basic";
    const category = result.find((candidate) => candidate.name === name);
    if (category) category.options.push(option);
    else result.push({ name, options: [option] });
    return result;
  }, []);
  const selectedOption = options.find((option) => option.value === value);
  const [open, setOpen] = useState(false);
  const [activeCategoryIndex, setActiveCategoryIndex] = useState(0);
  const [openCategory, setOpenCategory] = useState<string | null>(null);
  const [activeActionIndex, setActiveActionIndex] = useState(0);
  const [submenuFlipped, setSubmenuFlipped] = useState(false);
  const [popupPosition, setPopupPosition] = useState({ top: 0, left: 0 });
  const submenuCategory = categories.find((category) => category.name === openCategory);

  const restoreTriggerFocus = () => triggerRef.current?.focus();
  const closeSelector = () => {
    setOpen(false);
    setOpenCategory(null);
    restoreTriggerFocus();
  };

  useEffect(() => {
    if (!open) {
      setOpenCategory(null);
      setActiveCategoryIndex(0);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: MouseEvent) => {
      const target = event.target as Node;
      if (!containerRef.current?.contains(target) && !categoryMenuRef.current?.contains(target)) closeSelector();
    };
    document.addEventListener("mousedown", closeOnOutsideClick);
    return () => document.removeEventListener("mousedown", closeOnOutsideClick);
  }, [open]);

  useLayoutEffect(() => {
    if (!open || !triggerRef.current) return;
    const positionPopup = () => {
      const triggerBounds = triggerRef.current!.getBoundingClientRect();
      const popupHeight = categories.length * 37 + 8;
      setPopupPosition({
        top: Math.max(8, Math.min(triggerBounds.bottom + 4, window.innerHeight - popupHeight - 8)),
        left: Math.max(8, Math.min(triggerBounds.left, window.innerWidth - 168)),
      });
    };
    positionPopup();
    window.addEventListener("resize", positionPopup);
    window.addEventListener("scroll", positionPopup, true);
    return () => {
      window.removeEventListener("resize", positionPopup);
      window.removeEventListener("scroll", positionPopup, true);
    };
  }, [open, categories.length]);

  useLayoutEffect(() => {
    if (!openCategory || !categoryMenuRef.current) return;
    const { right } = categoryMenuRef.current.getBoundingClientRect();
    setSubmenuFlipped(right + 224 > window.innerWidth);
  }, [openCategory, popupPosition]);

  const openSelector = () => {
    setOpen(true);
    setActiveCategoryIndex(0);
    setOpenCategory(null);
  };

  const revealCategory = (index: number) => {
    setActiveCategoryIndex(index);
    setOpenCategory(categories[index]?.name ?? null);
    setActiveActionIndex(0);
  };

  const handleSelect = (option: GnomeSelectOption) => {
    if (option.disabled) return;
    onChange(option.value);
    closeSelector();
  };

  const moveCategory = (direction: number) => {
    setActiveCategoryIndex((previous) => (previous + direction + categories.length) % categories.length);
  };

  const moveAction = (direction: number) => {
    if (!submenuCategory) return;
    setActiveActionIndex((previous) => (previous + direction + submenuCategory.options.length) % submenuCategory.options.length);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (disabled) return;
    const action = submenuCategory?.options[activeActionIndex];

    if (event.key === "Escape") {
      event.preventDefault();
      if (openCategory) setOpenCategory(null);
      else if (open) closeSelector();
      return;
    }
    if (!open && ["Enter", " ", "ArrowDown", "ArrowUp"].includes(event.key)) {
      event.preventDefault();
      openSelector();
      return;
    }
    if (!open) return;

    if (event.key === "ArrowLeft" && openCategory) {
      event.preventDefault();
      setOpenCategory(null);
      return;
    }
    if (["ArrowRight", "Enter", " "].includes(event.key)) {
      event.preventDefault();
      if (openCategory && action) handleSelect(action);
      else revealCategory(activeCategoryIndex);
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const direction = event.key === "ArrowDown" ? 1 : -1;
      if (openCategory) moveAction(direction);
      else moveCategory(direction);
      return;
    }
    if (event.key === "Home" || event.key === "End") {
      event.preventDefault();
      if (openCategory) setActiveActionIndex(event.key === "Home" ? 0 : submenuCategory!.options.length - 1);
      else setActiveCategoryIndex(event.key === "Home" ? 0 : categories.length - 1);
    }
  };

  const activeDescendant = openCategory
    ? `${menuId}-action-${activeActionIndex}`
    : `${menuId}-category-${activeCategoryIndex}`;

  return (
    <div className={`gnome-select-wrapper ${className}`} ref={containerRef}>
      <button
        id={id}
        ref={triggerRef}
        type="button"
        className="select gnome-select-trigger"
        aria-label={ariaLabel}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-controls={open ? `${menuId}-categories` : undefined}
        aria-activedescendant={open ? activeDescendant : undefined}
        disabled={disabled}
        onClick={() => (open ? closeSelector() : openSelector())}
        onKeyDown={handleKeyDown}
      >
        <span className="gnome-select-value">{selectedOption?.label ?? placeholder ?? value}</span>
        <span className="gnome-select-arrow" aria-hidden="true">▾</span>
      </button>

      {open && createPortal(
        <div
          ref={categoryMenuRef}
          id={`${menuId}-categories`}
          className="gnome-select-popup"
          role="menu"
          aria-label={`${ariaLabel} categories`}
          style={popupPosition}
        >
          {categories.map((category, index) => (
            <div
              key={category.name}
              id={`${menuId}-category-${index}`}
              role="menuitem"
              aria-haspopup="menu"
              aria-expanded={openCategory === category.name}
              className={`gnome-select-option ${activeCategoryIndex === index ? "active" : ""}`}
              onClick={() => revealCategory(index)}
              onMouseEnter={() => revealCategory(index)}
            >
              {category.name}
              <span aria-hidden="true" className="gnome-select-submenu-arrow">▸</span>
            </div>
          ))}
          {submenuCategory && (
            <div className={`gnome-select-submenu ${submenuFlipped ? "flipped" : ""}`} role="menu" aria-label={`${submenuCategory.name} actions`}>
              {submenuCategory.options.map((option, index) => (
                <div
                  key={option.value}
                  id={`${menuId}-action-${index}`}
                  role="menuitem"
                  aria-disabled={option.disabled ?? false}
                  className={`gnome-select-option ${activeActionIndex === index ? "active" : ""} ${option.disabled ? "disabled" : ""}`}
                  onClick={() => handleSelect(option)}
                  onMouseEnter={() => setActiveActionIndex(index)}
                >
                  {option.label}
                </div>
              ))}
            </div>
          )}
        </div>,
        document.body,
      )}
    </div>
  );
}
