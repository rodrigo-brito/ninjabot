import { type ReactNode, useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";

interface Props {
  /** Name of the metric, shown as the title of the balloon. */
  label: string;
  children: ReactNode;
}

interface Placement {
  top: number;
  left: number;
}

const WIDTH = 300;
const GAP = 8;

/**
 * The "?" next to a metric. Hovering opens the balloon, clicking pins it open
 * so the text can be read (and selected) without holding the pointer still,
 * and Escape or a scroll closes it.
 *
 * The balloon is positioned in viewport coordinates instead of being anchored
 * in the flow: the metrics live inside grids that clip, and a tooltip that
 * gets cut off by its own card is worse than no tooltip.
 */
export function InfoTip({ label, children }: Props) {
  const [hovered, setHovered] = useState(false);
  const [pinned, setPinned] = useState(false);
  const [placement, setPlacement] = useState<Placement | null>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const balloon = useRef<HTMLSpanElement>(null);

  const open = hovered || pinned;

  const place = useCallback(() => {
    const rect = trigger.current?.getBoundingClientRect();
    if (!rect) return;
    setPlacement({
      top: rect.bottom + GAP,
      left: Math.min(
        Math.max(rect.left + rect.width / 2 - WIDTH / 2, GAP),
        window.innerWidth - WIDTH - GAP,
      ),
    });
  }, []);

  // Flip above the trigger when the balloon would run past the bottom.
  useLayoutEffect(() => {
    if (!open || !placement || !balloon.current || !trigger.current) return;
    const height = balloon.current.offsetHeight;
    if (placement.top + height <= window.innerHeight - GAP) return;

    const top = trigger.current.getBoundingClientRect().top - height - GAP;
    if (top > GAP && top !== placement.top) setPlacement({ ...placement, top });
  }, [open, placement]);

  useEffect(() => {
    if (!open) return;

    const close = () => {
      setHovered(false);
      setPinned(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!trigger.current?.contains(target) && !balloon.current?.contains(target)) close();
    };

    window.addEventListener("keydown", onKeyDown);
    window.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("scroll", close, true);
    window.addEventListener("resize", close);
    return () => {
      window.removeEventListener("keydown", onKeyDown);
      window.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("scroll", close, true);
      window.removeEventListener("resize", close);
    };
  }, [open]);

  return (
    <span className="infotip">
      <button
        ref={trigger}
        type="button"
        className={`infotip-trigger ${pinned ? "on" : ""}`}
        aria-label={`What ${label} means`}
        aria-expanded={open}
        onClick={() => {
          place();
          setPinned((current) => !current);
        }}
        onMouseEnter={() => {
          place();
          setHovered(true);
        }}
        onMouseLeave={() => setHovered(false)}
        onFocus={() => {
          place();
          setHovered(true);
        }}
        onBlur={() => setHovered(false)}
      >
        ?
      </button>
      {open && placement && (
        <span
          ref={balloon}
          className="infotip-balloon"
          role="tooltip"
          style={{ top: placement.top, left: placement.left, width: WIDTH }}
        >
          <strong>{label}</strong>
          {children}
        </span>
      )}
    </span>
  );
}
