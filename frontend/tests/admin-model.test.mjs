import test from 'node:test';
import assert from 'node:assert/strict';
import { draftGoal, goalFromDraft, validateDrafts } from '../src/admin-model.ts';
import { NEW_GOAL } from '../src/admin-config.ts';

test('admin drafts preserve editable metadata and tag input', () => {
  const goal = {
    ...NEW_GOAL,
    name: 'クリアする',
    world: 'ヨースターとう',
    level: 'コース4',
    tags: ['area:yoster', 'type:clear', 'type:restriction'],
  };
  const draft = draftGoal(goal, 42);
  assert.equal(draft.key, 42);
  assert.equal(draft.areas, 'yoster');
  assert.equal(draft.kinds, 'clear, restriction');
  assert.deepEqual(goalFromDraft(draft), goal);
  assert.deepEqual(goalFromDraft({ ...draft, areas: ' yoster, forest, ', kinds: 'clear,' }).tags, [
    'area:yoster',
    'area:forest',
    'type:clear',
  ]);
  assert.deepEqual(goal.tags, ['area:yoster', 'type:clear', 'type:restriction']);
});

test('admin validation catches invalid catalog edits before saving', () => {
  const drafts = Array.from({ length: 25 }, (_, index) =>
    draftGoal({ ...NEW_GOAL, name: `お題${index}` }, index),
  );
  assert.deepEqual(validateDrafts(drafts), []);
  assert.match(validateDrafts(drafts.slice(1)).join(''), /25件/);
  for (const [patch, expected] of [
    [{ name: '' }, /内容/],
    [{ name: 'お題1' }, /同じワールド/],
    [{ world: ' world' }, /空白/],
    [{ level: 'コース4' }, /ワールドも/],
    [{ timeMin: 0 }, /所要時間/],
    [{ timeMin: 1.5 }, /整数/],
    [{ timeMin: 1441 }, /所要時間/],
    [{ exec: 4 }, /操作難度/],
    [{ risk: 0 }, /リスク/],
    [{ areas: '' }, /タグ/],
    [{ kinds: 'a,b,c' }, /タグ/],
    [{ areas: 'a,a' }, /重複なし/],
  ]) {
    assert.match(
      validateDrafts(drafts.map((draft, index) => (index ? draft : { ...draft, ...patch }))).join(
        '',
      ),
      expected,
    );
  }
});
