import config from '../../backend/bingo/route_segments.json' with { type: 'json' };

export const ROUTE_SEGMENTS = config.segments;
export const FOREST_CONNECTIONS = config.forestConnections;
export const FOREST_SEGMENTS = config.forestSegments;
export const ALL_ROUTE_SEGMENTS = [...ROUTE_SEGMENTS, ...FOREST_SEGMENTS];
export const RETIRED_ROUTE_AREAS = config.retiredAreas;
export const ROUTE_SEGMENT_DESCRIPTIONS: Record<string, string> = Object.fromEntries(
  ALL_ROUTE_SEGMENTS.map((segment) => [segment.id, segment.description]),
);
export const ROUTE_SEGMENT_LABELS: Record<string, string> = Object.fromEntries(
  [...ALL_ROUTE_SEGMENTS, ...config.compatibilitySegments].map((segment) => [
    segment.id,
    segment.label,
  ]),
);

export function routeSegmentLabel(id: string): string {
  return Object.hasOwn(ROUTE_SEGMENT_LABELS, id) ? ROUTE_SEGMENT_LABELS[id] : id;
}
