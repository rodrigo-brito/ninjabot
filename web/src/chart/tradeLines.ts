import type { CanvasRenderingTarget2D } from "fancy-canvas";
import type {
  IPrimitivePaneRenderer,
  IPrimitivePaneView,
  ISeriesPrimitive,
  SeriesAttachedParameter,
  Time,
  UTCTimestamp,
} from "lightweight-charts";

/** A line between two (time, price) points drawn on the price pane. */
export interface TradeLine {
  fromTime: number;
  fromPrice: number;
  toTime: number;
  toPrice: number;
  color: string;
  dashed?: boolean;
}

interface Segment {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  color: string;
  dashed: boolean;
}

class TradeLinesRenderer implements IPrimitivePaneRenderer {
  constructor(private readonly segments: Segment[]) {}

  draw(target: CanvasRenderingTarget2D): void {
    if (this.segments.length === 0) return;
    // biome-ignore lint/correctness/useHookAtTopLevel: fancy-canvas API, not a React hook
    target.useMediaCoordinateSpace(({ context }) => {
      context.save();
      context.lineWidth = 1.5;
      for (const s of this.segments) {
        context.beginPath();
        context.strokeStyle = s.color;
        context.setLineDash(s.dashed ? [4, 4] : []);
        context.moveTo(s.x1, s.y1);
        context.lineTo(s.x2, s.y2);
        context.stroke();
      }
      context.restore();
    });
  }
}

class TradeLinesPaneView implements IPrimitivePaneView {
  private segments: Segment[] = [];

  constructor(private readonly source: TradeLinesPrimitive) {}

  update(): void {
    this.segments = this.source.segments();
  }

  renderer(): IPrimitivePaneRenderer {
    return new TradeLinesRenderer(this.segments);
  }
}

/**
 * Series primitive that draws straight lines between two points of the
 * price chart (e.g. the reference price of an OCO order and its target).
 */
export class TradeLinesPrimitive implements ISeriesPrimitive<Time> {
  private params?: SeriesAttachedParameter<Time>;
  private lines: TradeLine[] = [];
  private readonly view = new TradeLinesPaneView(this);

  attached(params: SeriesAttachedParameter<Time>): void {
    this.params = params;
  }

  detached(): void {
    this.params = undefined;
  }

  setLines(lines: TradeLine[]): void {
    this.lines = lines;
    this.params?.requestUpdate();
  }

  updateAllViews(): void {
    this.view.update();
  }

  paneViews(): readonly IPrimitivePaneView[] {
    return [this.view];
  }

  segments(): Segment[] {
    const params = this.params;
    if (!params) return [];
    const timeScale = params.chart.timeScale();
    const segments: Segment[] = [];
    for (const line of this.lines) {
      const x1 = timeScale.timeToCoordinate(line.fromTime as UTCTimestamp);
      const x2 = timeScale.timeToCoordinate(line.toTime as UTCTimestamp);
      const y1 = params.series.priceToCoordinate(line.fromPrice);
      const y2 = params.series.priceToCoordinate(line.toPrice);
      if (x1 === null || x2 === null || y1 === null || y2 === null) continue;
      segments.push({ x1, y1, x2, y2, color: line.color, dashed: line.dashed ?? false });
    }
    return segments;
  }
}
