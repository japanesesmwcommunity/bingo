import test from 'node:test';
import assert from 'node:assert/strict';
import {
  draftGoal,
  goalFromDraft,
  validateDrafts,
  draftRouteAreas,
  routeAreaTimesFromDraft,
} from '../src/admin-model.ts';
import { NEW_GOAL } from '../src/admin-config.ts';
import { ROUTE_SEGMENTS, routeSegmentLabel } from '../src/route-config.ts';
const segmentIds = ROUTE_SEGMENTS.map((segment) => segment.id);
const completeTimes = Object.fromEntries(segmentIds.map((name) => [name, 1]));

test('route metadata round-trips independently of placement tags', () => {
  const goal = { ...NEW_GOAL, routeAreas: ['a', 'b', 'c'] };
  const draft = draftGoal(goal, 0);
  assert.deepEqual(draft.routes, ['a', 'b', 'c']);
  assert.deepEqual(goalFromDraft(draft), goal);
  draft.routes.push('d');
  assert.deepEqual(goal.routeAreas, ['a', 'b', 'c']);
  const saved = goalFromDraft(draft);
  saved.routeAreas.push('e');
  assert.deepEqual(draft.routes, ['a', 'b', 'c', 'd']);
  assert.equal(goalFromDraft({ ...draft, routes: [] }).routeAreas, undefined);
  const times = { a: 1, b: 2, c: 3 };
  assert.deepEqual(routeAreaTimesFromDraft(draftRouteAreas(times)), times);
  const blank = draftRouteAreas();
  assert.deepEqual(
    blank.map((area) => area.name),
    [...segmentIds],
  );
  assert.equal(
    blank.every((area) => area.timeMin === ''),
    true,
  );
  assert.deepEqual(routeAreaTimesFromDraft(blank), {});
  const existing = draftRouteAreas({ ヨースターとう: 4, 独自区間: 2 }, [
    '追加区間',
    'ヨースターとう',
  ]);
  assert.equal(existing.length, segmentIds.length + 2);
  assert.equal(existing.find((area) => area.name === 'ヨースターとう').timeMin, 4);
  assert.equal(existing.find((area) => area.name === '追加区間').timeMin, '');
  assert.deepEqual(routeAreaTimesFromDraft(existing), { ヨースターとう: 4, 独自区間: 2 });
  assert.equal(
    JSON.stringify(routeAreaTimesFromDraft(draftRouteAreas({ z: 2, a: 1 }))),
    JSON.stringify(routeAreaTimesFromDraft(draftRouteAreas({ a: 1, z: 2 }))),
  );
});

test('route validation prevents missing references duplicates and impossible travel totals', () => {
  const goals = Array.from({ length: 25 }, (_, i) =>
    draftGoal({ ...NEW_GOAL, name: `goal${i}`, routeAreas: ['a', 'b', 'c'] }, i),
  );
  const areas = draftRouteAreas({ ...completeTimes, a: 1, b: 1, c: 3 });
  assert.deepEqual(validateDrafts(goals, areas), []);
  for (const [drafts, routes, expected] of [
    [goals, [], /未登録/],
    [goals, draftRouteAreas(), /未設定/],
    [goals, [...areas, areas[0]], /重複/],
    [goals, draftRouteAreas({ a: 1, b: 1, c: 4 }), /超えています/],
    [goals, draftRouteAreas({ a: 0 }), /整数/],
    [goals, draftRouteAreas({ a: 1.5 }), /整数/],
    [goals, draftRouteAreas({ a: 1441 }), /整数/],
    [goals, draftRouteAreas({ ' a': 1 }), /区間名/],
    [goals, draftRouteAreas({ 'a,b': 1 }), /区間名/],
    [goals, draftRouteAreas({ '': 1 }), /区間名/],
    [goals.map((g) => ({ ...g, routes: ['a', 'a'] })), areas, /重複/],
    [goals.map((g) => ({ ...g, routes: ['toString'] })), areas, /未登録/],
  ])
    assert.match(validateDrafts(drafts, routes).join('\n'), expected);
});

test('branch segments stay distinct and legacy totals are preserved without guessing splits', () => {
  assert.equal(
    routeSegmentLabel('vanilla:cheese'),
    'バニラドーム：コース1終了後からチーズブリッジへ',
  );
  assert.equal(routeSegmentLabel('unknown'), 'unknown');
  assert.equal(routeSegmentLabel('toString'), 'toString');
  const times = { ...completeTimes, バニラドーム: 10, バニラだいち: 5 };
  const areas = draftRouteAreas(times);
  assert.equal(areas.find((area) => area.name === 'バニラドーム').timeMin, 10);
  assert.equal(
    draftRouteAreas({ バニラドーム: 10 }).find((area) => area.name === 'vanilla:common').timeMin,
    '',
  );
  assert.deepEqual(routeAreaTimesFromDraft(areas), times);
  const goals = Array.from({ length: 25 }, (_, i) =>
    draftGoal({ ...NEW_GOAL, name: `goal${i}` }, i),
  );
  goals[0].routes = ['vanilla:common', 'vanilla:cheese'];
  goals[1].routes = ['vanilla:common', 'vanilla:plateau', 'plateau:butter'];
  assert.deepEqual(validateDrafts(goals, areas), []);
  assert.deepEqual(goalFromDraft(goals[0]).routeAreas, ['vanilla:common', 'vanilla:cheese']);
  goals[0].routes = ['バニラドーム'];
  assert.match(validateDrafts(goals, areas).join('\n'), /旧エリア.*選び直して/);
});
