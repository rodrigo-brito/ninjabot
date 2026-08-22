import { useId } from "react";
import type { Point } from "../api/types";

interface Props {
  points: Point[];
  /** CSS color of the line and glow. */
  color: string;
  width?: number;
  height?: number;
  className?: string;
}

/** Downsamples to at most `max` points, keeping the first and last. */
function sample(points: Point[], max: number): Point[] {
  if (points.length <= max) return points;
  const step = (points.length - 1) / (max - 1);
  const out: Point[] = [];
  for (let i = 0; i < max; i++) {
    const p = points[Math.round(i * step)];
    if (p) out.push(p);
  }
  return out;
}

/** Tiny inline SVG line chart used as the signature of the hero stat. */
export function Sparkline({ points, color, width = 320, height = 64, className }: Props) {
  const id = useId();
  const data = sample(points, 160);
  if (data.length < 2) return null;

  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  for (const p of data) {
    if (p.value < min) min = p.value;
    if (p.value > max) max = p.value;
  }
  const span = max - min || 1;
  const pad = 4;
  const innerH = height - pad * 2;
  const coords = data.map((p, i) => {
    const x = (i / (data.length - 1)) * width;
    const y = pad + innerH - ((p.value - min) / span) * innerH;
    return [x, y] as const;
  });
  const line = coords
    .map(([x, y], i) => `${i === 0 ? "M" : "L"}${x.toFixed(1)} ${y.toFixed(1)}`)
    .join(" ");
  const area = `${line} L${width} ${height} L0 ${height} Z`;

  return (
    <svg
      className={className}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      aria-hidden="true"
      focusable="false"
    >
      <defs>
        <linearGradient id={`${id}-fill`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.28" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      <path d={area} fill={`url(#${id}-fill)`} />
      <path
        d={line}
        fill="none"
        stroke={color}
        strokeWidth="1.5"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
