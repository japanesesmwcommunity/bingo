import config from '../../backend/bingo/route_segments.json' with { type: 'json' };

export const ROUTE_SEGMENTS = config.segments;
export const RETIRED_ROUTE_AREAS = config.retiredAreas;
export const ROUTE_SEGMENT_LABELS: Record<string, string> = Object.fromEntries(
  ROUTE_SEGMENTS.map((segment) => [segment.id, segment.label]),
);

export function routeSegmentLabel(id: string): string {
  return Object.hasOwn(ROUTE_SEGMENT_LABELS, id) ? ROUTE_SEGMENT_LABELS[id] : id;
}
