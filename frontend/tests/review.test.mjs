import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import {
  draftGoal,
  goalFromDraft,
  draftRouteAreas,
  validateDrafts,
  validateFinishRoutes,
} from '../src/admin-model.ts';
import { NEW_GOAL } from '../src/admin-config.ts';

test('review metadata survives editing without aliases or empty optional fields', () => {
  const goal = {
    ...NEW_GOAL,
    name: 'pending',
    disabledReason: '条件未確定',
    conflictGroups: ['nested'],
  };
  const draft = draftGoal(goal, 0);
  assert.equal(draft.groups, 'nested');
  draft.groups = 'nested, other';
  assert.deepEqual(goal.conflictGroups, ['nested']);
  assert.deepEqual(goalFromDraft(draft).conflictGroups, ['nested', 'other']);
  assert.equal(goalFromDraft(draft).disabledReason, '条件未確定');
  const cleared = goalFromDraft({ ...draft, groups: '', disabledReason: '' });
  assert.equal(cleared.disabledReason, undefined);
  assert.equal(cleared.conflictGroups, undefined);
});

test('validation counts active goals and permits unused unmeasured route segments', async () => {
  const catalog = JSON.parse(
    await readFile(new URL('../../backend/bingo.json', import.meta.url), 'utf8'),
  );
  const drafts = catalog.goals.map(draftGoal);
  assert.equal(drafts.filter((g) => !g.disabledReason).length, 40);
  assert.deepEqual(validateDrafts(drafts, draftRouteAreas(catalog.routeAreaTimes)), []);
  const fixture = Array.from({ length: 26 }, (_, i) =>
    draftGoal({ ...NEW_GOAL, name: `goal-${i}` }, i),
  );
  fixture[0].disabledReason = 'pending';
  assert.deepEqual(validateDrafts(fixture, draftRouteAreas()), []);
  fixture[1].disabledReason = 'pending';
  assert.match(validateDrafts(fixture).join(), /25件/);
  fixture[1].disabledReason = '';
  fixture[1].groups = 'a, a';
  assert.match(validateDrafts(fixture).join(), /グループ/);
  fixture[1].groups = '';
  fixture[1].disabledReason = ' ';
  assert.match(validateDrafts(fixture).join(), /保留理由/);
});

test('finish-route validation requires a distinct named path with residual fight cost', () => {
  const times = { shared: 4, ドーナツへいや: 4 };
  const valid = { name: 'star', timeMin: 6, routeAreas: ['shared'] };
  assert.deepEqual(validateFinishRoutes([], times), []);
  assert.deepEqual(validateFinishRoutes([valid], times), []);
  for (const patch of [
    { name: '' },
    { name: ' spaced' },
    { timeMin: 0 },
    { timeMin: 1.5 },
    { timeMin: 1441 },
    { routeAreas: ['missing'] },
    { routeAreas: ['shared', 'shared'] },
    { routeAreas: ['ドーナツへいや'] },
    { timeMin: 4 },
  ])
    assert.ok(validateFinishRoutes([{ ...valid, ...patch }], times).length, JSON.stringify(patch));
  assert.ok(validateFinishRoutes([valid, valid], times).length);
});
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { BowserRouteEditor } from '../src/components/BowserRouteEditor.tsx';

function elements(root) {
  if (!root || typeof root !== 'object') return [];
  if (Array.isArray(root)) return root.flatMap(elements);
  return [root, ...elements(root.props?.children)];
}

test('finish-route editor displays the fallback and edits every route field', () => {
  let result;
  const routes = [{ name: 'star', timeMin: 12, routeAreas: ['shared'] }];
  const props = {
    routes,
    areas: [
      { key: 0, name: 'shared', timeMin: 8 },
      { key: 1, name: 'unused', timeMin: '' },
      { key: 2, name: 'ドーナツへいや', timeMin: 5 },
    ],
    pending: false,
    onChange: (r) => {
      result = r;
    },
  };
  const tree = BowserRouteEditor(props);
  const all = elements(tree);
  const markup = renderToStaticMarkup(React.createElement(BowserRouteEditor, props));
  assert.match(markup, /従来の固定時間/);
  assert.equal(all.filter((e) => e.type === 'option').length, 1);
  const inputs = all.filter((e) => e.type === 'input');
  inputs[0].props.onChange({ target: { value: 'back' } });
  assert.equal(result[0].name, 'back');
  inputs[1].props.onChange({ target: { value: '15' } });
  assert.equal(result[0].timeMin, 15);
  all.find((e) => e.type === 'select').props.onChange({ target: { selectedOptions: [] } });
  assert.deepEqual(result[0].routeAreas, []);
  const buttons = all.filter((e) => e.type === 'button');
  buttons[0].props.onClick();
  assert.deepEqual(result, []);
  buttons[1].props.onClick();
  assert.equal(result.length, 2);
  assert.equal(routes[0].name, 'star');
  assert.equal(
    elements(BowserRouteEditor({ ...props, pending: true })).find((e) => e.type === 'fieldset')
      .props.disabled,
    true,
  );
  assert.equal(BowserRouteEditor({ ...props, routes: [] }).props.open, false);
  assert.match(markup, /通常15分/);
  assert.match(markup, /現在地からの残り時間ではありません/);
});
