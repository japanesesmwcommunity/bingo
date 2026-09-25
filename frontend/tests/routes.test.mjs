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
import {
  FOREST_CONNECTIONS,
  FOREST_SEGMENTS,
  ALL_ROUTE_SEGMENTS,
  ROUTE_SEGMENTS,
  routeSegmentLabel,
} from '../src/route-config.ts';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { RouteAreaEditor } from '../src/components/RouteAreaEditor.tsx';
const segmentIds = ALL_ROUTE_SEGMENTS.map((segment) => segment.id);
const completeTimes = Object.fromEntries(segmentIds.map((name) => [name, 1]));

test('forest connections separate castle access, fortress access and return paths', () => {
  assert.deepEqual(
    FOREST_CONNECTIONS.map(({ course, normal, secret }) => [course, normal, secret]),
    [
      ['コース1', 'コース2', 'おばけやしき'],
      ['コース2', 'コース3', '青スイッチのきゅうでん'],
      ['コース3', 'おばけやしき', '城5'],
      ['おばけやしき', 'コース4', 'コース1へ戻る'],
      ['コース4', 'コース2へ戻る', 'ひみつのコース'],
      ['ひみつのコース', 'とりで', null],
      ['とりで', 'スターロード（ネイティブスター4側）', null],
      ['城5', 'チョコレーとう コース1', null],
    ],
  );
  assert.ok(FOREST_CONNECTIONS.every((c) => c.source.startsWith('https://')));
});

test('course groups distinguish exits without double-counting shared partial courses', () => {
  const exits = ROUTE_SEGMENTS.flatMap((segment) => segment.exits);
  assert.equal(new Set(exits).size, exits.length);
  const lower = ROUTE_SEGMENTS.find((segment) => segment.id === 'unlock:vanilla-lower');
  const upper = ROUTE_SEGMENTS.find((segment) => segment.id === 'unlock:vanilla-upper');
  assert.ok(lower.exits.includes('vanilla:1:normal'));
  assert.ok(upper.exits.includes('vanilla:1:secret'));
  assert.deepEqual(ROUTE_SEGMENTS.find((s) => s.id === 'unlock:valley-back').exits, [
    'valley:2:secret',
    'valley-fortress:normal',
  ]);
  assert.deepEqual(ROUTE_SEGMENTS.find((s) => s.id === 'unlock:star-front').exits, [
    'star:1:secret',
    'star:2:secret',
    'star:3:secret',
    'star:4:secret',
  ]);
  assert.ok(ROUTE_SEGMENTS.every((s) => s.description && s.source.startsWith('https://')));
});

function elements(root) {
  if (!root || typeof root !== 'object') return [];
  if (Array.isArray(root)) return root.flatMap(elements);
  return [root, ...elements(root.props?.children)];
}

test('route editor hides old values, preserves them on edit and explains course boundaries', () => {
  let result;
  const areas = draftRouteAreas({ 'forest:castle': 2 });
  const props = {
    areas,
    pending: false,
    onChange: (value) => {
      result = value;
    },
  };
  const tree = RouteAreaEditor(props);
  assert.equal(Boolean(tree.props.open), false);
  const all = elements(tree);
  const inputs = all.filter((e) => e.type === 'input');
  assert.equal(inputs.length, 8 + FOREST_SEGMENTS.length);
  inputs.find((e) => e.props.name === 'route-area-time').props.onChange({ target: { value: '4' } });
  assert.equal(result[0].timeMin, 4);
  assert.equal(result.find((a) => a.name === 'forest:castle').timeMin, 2);
  inputs[0].props.onChange({ target: { value: '' } });
  assert.equal(result[0].timeMin, '');
  assert.equal(areas[0].timeMin, '');
  const markup = renderToStaticMarkup(React.createElement(RouteAreaEditor, props));
  assert.match(markup, /途中までのお題には選ばない/);
  assert.doesNotMatch(markup, /旧設定の参考値|legacy-route-times|森3隠しゴールから城5へ/);
  assert.match(markup, /コース2隠しゴール→とりで攻略/);
  assert.doesNotMatch(markup, /出口/);
  assert.match(markup, /同じゴールの攻略時間だけを重複補正/);
  assert.match(markup, /森2→3→おばけやしき→4→2/);
  assert.equal(
    elements(RouteAreaEditor({ ...props, pending: true })).find((e) => e.type === 'fieldset').props
      .disabled,
    true,
  );
});

test('default route settings omit speculative branches and preserve saved settings', () => {
  const blank = draftRouteAreas();
  assert.equal(blank.length, 8 + FOREST_SEGMENTS.length);
  for (const prefix of ['forest:', 'donut:', 'choco:', 'valley:', 'star:']) {
    assert.equal(
      blank.some((area) => area.name.startsWith(prefix)),
      false,
    );
  }
  const times = { 'forest:castle': 2, 'star:1-secret': 3 };
  assert.deepEqual(routeAreaTimesFromDraft(draftRouteAreas(times)), times);
  assert.equal(routeSegmentLabel('forest:castle'), '森3隠しゴールから城5へ');
  const referenced = draftRouteAreas({}, ['forest:castle']);
  assert.equal(referenced.find((area) => area.name === 'forest:castle').timeMin, '');
});

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
  assert.equal(existing.length, segmentIds.length + 3);
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
    draftRouteAreas({ バニラドーム: 10 }).find((area) => area.name === 'unlock:vanilla-lower')
      .timeMin,
    '',
  );
  assert.deepEqual(routeAreaTimesFromDraft(areas), times);
  const goals = Array.from({ length: 25 }, (_, i) =>
    draftGoal({ ...NEW_GOAL, name: `goal${i}` }, i),
  );
  goals[0].routes = ['unlock:vanilla-lower'];
  goals[1].routes = ['unlock:vanilla-upper', 'unlock:butter'];
  assert.deepEqual(validateDrafts(goals, areas), []);
  assert.deepEqual(goalFromDraft(goals[0]).routeAreas, ['unlock:vanilla-lower']);
  goals[0].routes = ['バニラドーム'];
  assert.match(validateDrafts(goals, areas).join('\n'), /旧エリア.*選び直して/);
});
