import test from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { forestPathsTo, replaceForestPath } from '../src/forest-routes.ts';
import { FOREST_SEGMENTS } from '../src/route-config.ts';
import { ForestRoutePicker } from '../src/components/ForestRoutePicker.tsx';
import { RouteAreaEditor } from '../src/components/RouteAreaEditor.tsx';
import { draftRouteAreas, draftGoal, validateDrafts } from '../src/admin-model.ts';
import { NEW_GOAL } from '../src/admin-config.ts';

test('forest paths handle shortcuts, both castle approaches and loops', () => {
  assert.deepEqual(forestPathsTo('unknown'), []);
  assert.deepEqual(forestPathsTo('コース1')[0].segments, []);
  const house = forestPathsTo('おばけやしき').map((p) => p.segments);
  assert.deepEqual(house, [
    ['unlock:forest:1:secret'],
    ['unlock:forest:1:normal', 'unlock:forest:2:normal', 'unlock:forest:3:normal'],
  ]);
  assert.ok(forestPathsTo('城5').every((p) => p.segments.at(-1) === 'unlock:forest:3:secret'));
  assert.ok(
    forestPathsTo('とりで').every((p) => p.segments.at(-1) === 'unlock:forest:secret:normal'),
  );
  for (const target of new Set(FOREST_SEGMENTS.map((e) => e.to))) {
    for (const path of forestPathsTo(target)) {
      const visited = new Set(['コース1']);
      let node = 'コース1';
      for (const id of path.segments) {
        const edge = FOREST_SEGMENTS.find((e) => e.id === id);
        assert.equal(edge.from, node);
        assert.equal(visited.has(edge.to), false);
        node = edge.to;
        visited.add(node);
      }
      assert.equal(node, target);
    }
  }
});

function elements(root) {
  if (!root || typeof root !== 'object') return [];
  if (Array.isArray(root)) return root.flatMap(elements);
  return [root, ...elements(root.props?.children)];
}

test('picker replaces only forest clears and never fabricates times', () => {
  const original = ['unlock:yoster', 'unlock:forest:1:normal'];
  const path = forestPathsTo('おばけやしき')[0];
  assert.deepEqual(replaceForestPath(original, path), ['unlock:yoster', 'unlock:forest:1:secret']);
  let result;
  const tree = ForestRoutePicker({
    routes: original,
    onChange: (value) => {
      result = value;
    },
  });
  const all = elements(tree);
  const select = all.find((e) => e.type === 'select');
  const option = all.find((e) => e.type === 'option' && e.props.children === path.label);
  select.props.onChange({ target: { value: String(option.props.value) } });
  assert.deepEqual(result, ['unlock:yoster', 'unlock:forest:1:secret']);
  assert.deepEqual(original, ['unlock:yoster', 'unlock:forest:1:normal']);
  assert.match(
    renderToStaticMarkup(React.createElement(ForestRoutePicker, { routes: [], onChange() {} })),
    /目的地のコース自体の攻略/,
  );
  const goals = Array.from({ length: 25 }, (_, i) =>
    draftGoal({ ...NEW_GOAL, name: `goal${i}`, timeMin: 20 }, i),
  );
  goals[0].routes = path.segments;
  assert.match(validateDrafts(goals, draftRouteAreas()).join(), /未設定/);
  assert.deepEqual(validateDrafts(goals, draftRouteAreas({ 'unlock:forest:1:secret': 3 })), []);
});

test('forest time inputs retain unrelated settings and can be cleared', () => {
  const areas = draftRouteAreas({ 'unlock:yoster': 4 });
  let result;
  const tree = RouteAreaEditor({
    areas,
    pending: false,
    onChange: (value) => {
      result = value;
    },
  });
  const input = elements(tree).find((e) => e.props?.name === 'forest-goal-time');
  input.props.onChange({ target: { value: '2' } });
  assert.equal(result.find((a) => a.name === FOREST_SEGMENTS[0].id).timeMin, 2);
  assert.equal(result.find((a) => a.name === 'unlock:yoster').timeMin, 4);
  input.props.onChange({ target: { value: '' } });
  assert.equal(result.find((a) => a.name === FOREST_SEGMENTS[0].id).timeMin, '');
});
