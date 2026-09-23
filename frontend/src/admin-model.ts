import type { Goal, FinishRoute } from './types';
import type { DraftGoal, DraftRouteArea } from './admin-types';
import { ROUTE_SEGMENTS, RETIRED_ROUTE_AREAS, routeSegmentLabel } from './route-config';

export function draftGoal(goal: Goal, key: number): DraftGoal {
  const { tags, routeAreas, conflictGroups, ...fields } = goal;
  return {
    ...fields,
    groups: (conflictGroups ?? []).join(', '),
    key,
    routes: [...(routeAreas ?? [])],
    areas: tags
      .filter((tag) => tag.startsWith('area:'))
      .map((tag) => tag.slice(5))
      .join(', '),
    kinds: tags
      .filter((tag) => tag.startsWith('type:'))
      .map((tag) => tag.slice(5))
      .join(', '),
  };
}

export function goalFromDraft(draft: DraftGoal): Goal {
  const { key: _key, areas, kinds, routes, groups, disabledReason, ...goal } = draft;
  const conflictGroups = groups
    .split(',')
    .map((g) => g.trim())
    .filter(Boolean);
  const routeAreas = [...routes];
  return {
    ...goal,
    ...(disabledReason ? { disabledReason } : {}),
    ...(conflictGroups.length ? { conflictGroups } : {}),
    ...(routeAreas.length ? { routeAreas } : {}),
    tags: [
      ...areas
        .split(',')
        .map((tag) => tag.trim())
        .filter(Boolean)
        .map((tag) => `area:${tag}`),
      ...kinds
        .split(',')
        .map((tag) => tag.trim())
        .filter(Boolean)
        .map((tag) => `type:${tag}`),
    ],
  };
}

export function validateDrafts(drafts: DraftGoal[], routeAreas: DraftRouteArea[] = []): string[] {
  const errors: string[] = [];
  const names = new Set<string>();
  for (const area of routeAreas) {
    if (RETIRED_ROUTE_AREAS.includes(area.name)) continue;
    if (!area.name || area.name !== area.name.trim() || area.name.includes(','))
      errors.push('経由区間名は空欄・前後の空白・カンマを含めずに入力してください。');
    if (names.has(area.name))
      errors.push(`経由区間「${routeSegmentLabel(area.name)}」が重複しています。`);
    names.add(area.name);
    if (area.timeMin === '') continue;
    if (!Number.isInteger(area.timeMin) || area.timeMin < 1 || area.timeMin > 1440)
      errors.push(
        `経由区間「${routeSegmentLabel(area.name)}」の基準時間は1〜1440分の整数にしてください。`,
      );
  }
  const routeTimes = routeAreaTimesFromDraft(routeAreas);
  if (drafts.filter((goal) => !goal.disabledReason).length < 25)
    errors.push('抽選対象のお題は25件以上必要です（保留中を除く）。');
  const identities = new Set<string>();
  for (const [index, draft] of drafts.entries()) {
    const goal = goalFromDraft(draft);
    const label = `${index + 1}件目${goal.name ? `「${goal.name}」` : ''}`;
    const identity = JSON.stringify([goal.world, goal.level, goal.name.trim()]);
    if (!goal.name.trim()) errors.push(`${label}: お題の内容を入力してください。`);
    if (identities.has(identity))
      errors.push(`${label}: 同じワールド・コース・内容のお題があります。`);
    identities.add(identity);
    if (goal.world !== goal.world.trim() || goal.level !== goal.level.trim())
      errors.push(`${label}: ワールド・コースの前後の空白を除いてください。`);
    if (!goal.world && goal.level)
      errors.push(`${label}: コースを指定する場合はワールドも入力してください。`);
    if (!Number.isInteger(goal.timeMin) || goal.timeMin < 1 || goal.timeMin > 1440)
      errors.push(`${label}: 所要時間は1〜1440分の整数にしてください。`);
    if (![1, 2, 3].includes(goal.exec) || ![1, 2, 3].includes(goal.risk))
      errors.push(`${label}: 操作難度・リスクは1〜3で指定してください。`);
    if (goal.disabledReason && goal.disabledReason !== goal.disabledReason.trim())
      errors.push(label + ': 保留理由の前後の空白を除いてください。');
    const groups = goal.conflictGroups ?? [];
    if (
      new Set(groups).size !== groups.length ||
      groups.some((g) => !g || g !== g.trim() || g.includes(','))
    )
      errors.push(label + ': 同時配置禁止グループの空欄・空白・重複を確認してください。');
    const routes = goal.routeAreas ?? [];
    for (const route of routes)
      if (!goal.disabledReason && RETIRED_ROUTE_AREAS.includes(route))
        errors.push(
          `${label}: 旧エリア「${route}」を、実際に通る共通区間・分岐区間へ選び直してください。`,
        );
    if (new Set(routes).size !== routes.length) errors.push(`${label}: 経由区間が重複しています。`);
    for (const area of routes)
      if (!Object.hasOwn(routeTimes, area))
        errors.push(`${label}: 未登録または基準時間が未設定の経由区間です: ${area}`);
    if (
      routes.reduce(
        (sum, area) => sum + (Object.hasOwn(routeTimes, area) ? routeTimes[area] : 0),
        0,
      ) > goal.timeMin
    )
      errors.push(`${label}: 経由区間の合計時間が最低所要時間を超えています。`);
    const areas = goal.tags.filter((tag) => tag.startsWith('area:'));
    const kinds = goal.tags.filter((tag) => tag.startsWith('type:'));
    if (
      areas.length < 1 ||
      areas.length > 2 ||
      kinds.length < 1 ||
      kinds.length > 2 ||
      new Set(goal.tags).size !== goal.tags.length
    )
      errors.push(`${label}: エリア・種類タグはそれぞれ重複なしで1〜2個指定してください。`);
  }
  return errors;
}

export function draftRouteAreas(
  times: Record<string, number> = {},
  extraAreas: string[] = [],
): DraftRouteArea[] {
  const names = [
    ...new Set([
      ...ROUTE_SEGMENTS.map((segment) => segment.id),
      ...Object.keys(times),
      ...extraAreas,
    ]),
  ];
  return names.map((name, key) => ({
    key,
    name,
    timeMin: Object.hasOwn(times, name) ? times[name] : '',
  }));
}

export function routeAreaTimesFromDraft(areas: DraftRouteArea[]): Record<string, number> {
  return Object.fromEntries(
    [...areas]
      .filter((area): area is DraftRouteArea & { timeMin: number } => area.timeMin !== '')
      .sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
      .map((area) => [area.name, area.timeMin]),
  );
}

export function validateFinishRoutes(
  routes: FinishRoute[],
  times: Record<string, number>,
): string[] {
  const errors: string[] = [];
  const names = new Set<string>();
  for (const route of routes) {
    if (!route.name || route.name !== route.name.trim() || names.has(route.name))
      errors.push('クッパ経路名の空欄・空白・重複を確認してください。');
    names.add(route.name);
    if (!Number.isInteger(route.timeMin) || route.timeMin < 1 || route.timeMin > 1440)
      errors.push('クッパ経路の所要時間は1〜1440分の整数にしてください。');
    const areas = route.routeAreas ?? [];
    if (
      new Set(areas).size !== areas.length ||
      areas.some((a) => !Object.hasOwn(times, a) || RETIRED_ROUTE_AREAS.includes(a))
    )
      errors.push('クッパ経路の経由区間に未登録・重複・旧エリアがあります。');
    if (
      areas.reduce((sum, a) => sum + (Object.hasOwn(times, a) ? times[a] : 0), 0) >= route.timeMin
    )
      errors.push('クッパ経路には移動・再入場・戦闘の時間を残してください。');
  }
  return errors;
}
