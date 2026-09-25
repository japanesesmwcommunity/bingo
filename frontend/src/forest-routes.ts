import { FOREST_SEGMENTS, routeSegmentLabel } from './route-config';

export interface ForestPath {
  target: string;
  segments: string[];
  label: string;
}

// Enumerate simple unlock paths from the forest entrance. A return connection
// never unlocks a new destination on the same path, so revisits are excluded.
export function forestPathsTo(target: string): ForestPath[] {
  const queue = [{ node: 'コース1', segments: [] as string[], visited: ['コース1'] }];
  const paths: ForestPath[] = [];
  while (queue.length) {
    const current = queue.shift()!;
    if (current.node === target) {
      paths.push({
        target,
        segments: current.segments,
        label:
          current.segments
            .map((id) => routeSegmentLabel(id).replace('まよいのもり ', ''))
            .join(' → ') || '森1までの道中のみ',
      });
      continue;
    }
    for (const edge of FOREST_SEGMENTS) {
      if (edge.from !== current.node || current.visited.includes(edge.to)) continue;
      queue.push({
        node: edge.to,
        segments: [...current.segments, edge.id],
        visited: [...current.visited, edge.to],
      });
    }
  }
  return paths;
}

export function replaceForestPath(routes: string[], path: ForestPath): string[] {
  const forestIDs = new Set(FOREST_SEGMENTS.map((segment) => segment.id));
  return [...routes.filter((id) => !forestIDs.has(id)), ...path.segments];
}
