import { useEffect, useState } from 'react';
import { api, ApiError } from '../api';
import { NEW_GOAL } from '../admin-config';
import { WORLD_NAMES } from '../stage-config';
import { RETIRED_ROUTE_AREAS, routeSegmentLabel } from '../route-config';
import {
  draftGoal,
  goalFromDraft,
  validateDrafts,
  validateFinishRoutes,
  draftRouteAreas,
  routeAreaTimesFromDraft,
} from '../admin-model';
import type { CatalogDocument, DraftGoal, DraftRouteArea } from '../admin-types';
import type { FinishRoute } from '../types';
import { BowserRouteEditor } from './BowserRouteEditor';
import { RouteAreaEditor } from './RouteAreaEditor';

export function GoalEditor() {
  const [saved, setSaved] = useState<CatalogDocument | null>(null);
  const [drafts, setDrafts] = useState<DraftGoal[]>([]);
  const [routeAreas, setRouteAreas] = useState<DraftRouteArea[]>([]);
  const [bowserRoutes, setBowserRoutes] = useState<FinishRoute[]>([]);
  const [selected, setSelected] = useState(0);
  const [search, setSearch] = useState('');
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState('');
  const [error, setError] = useState('');
  const [needsLogin, setNeedsLogin] = useState(false);
  const dirty = Boolean(
    saved &&
    (JSON.stringify(bowserRoutes) !== JSON.stringify(saved.bowserRoutes ?? []) ||
      JSON.stringify(drafts.map(goalFromDraft)) !==
        JSON.stringify(saved.goals.map(draftGoal).map(goalFromDraft)) ||
      JSON.stringify(routeAreaTimesFromDraft(routeAreas)) !==
        JSON.stringify(routeAreaTimesFromDraft(draftRouteAreas(saved.routeAreaTimes)))),
  );
  const errors = [
    ...validateDrafts(drafts, routeAreas),
    ...validateFinishRoutes(bowserRoutes, routeAreaTimesFromDraft(routeAreas)),
  ];
  const current = drafts.find((goal) => goal.key === selected);
  const filtered = drafts.filter((goal) =>
    `${goal.world} ${goal.level} ${goal.name}`
      .toLocaleLowerCase()
      .includes(search.toLocaleLowerCase()),
  );
  const worlds = [
    ...new Set([...WORLD_NAMES, ...drafts.map((goal) => goal.world).filter(Boolean)]),
  ];

  const availableRoutes = routeAreas.filter(
    (area) => !RETIRED_ROUTE_AREAS.includes(area.name) && !current?.routes.includes(area.name),
  );

  useEffect(() => {
    let active = true;
    void api<CatalogDocument>('/api/admin/goals')
      .then((data) => {
        if (active) {
          setSaved(data);
          setBowserRoutes(data.bowserRoutes ?? []);
          setDrafts(data.goals.map(draftGoal));
          setRouteAreas(
            draftRouteAreas(
              data.routeAreaTimes,
              data.goals.flatMap((goal) => goal.routeAreas ?? []).filter(Boolean),
            ),
          );
        }
      })
      .catch((error) => {
        if (active) {
          setError(String(error));
          setNeedsLogin(error instanceof ApiError && error.status === 401);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [dirty]);

  function change(patch: Partial<DraftGoal>) {
    setDrafts((goals) =>
      goals.map((goal) => (goal.key === selected ? { ...goal, ...patch } : goal)),
    );
    setNotice('');
  }

  function add() {
    const key = Math.max(-1, ...drafts.map((goal) => goal.key)) + 1;
    setDrafts([...drafts, draftGoal(NEW_GOAL, key)]);
    setSelected(key);
    setSearch('');
    setNotice('');
  }

  function remove() {
    const remaining = drafts.filter((goal) => goal.key !== selected);
    setDrafts(remaining);
    setSelected(remaining[0]?.key ?? 0);
    setNotice('');
  }

  async function save() {
    if (!saved || errors.length || pending) return;
    setPending(true);
    setError('');
    setNotice('');
    try {
      const data = await api<CatalogDocument>('/api/admin/goals', 'PUT', {
        revision: saved.revision,
        bowserRoutes,
        goals: drafts.map(goalFromDraft),
        routeAreaTimes: routeAreaTimesFromDraft(routeAreas),
      });
      setSaved(data);
      setBowserRoutes(data.bowserRoutes ?? []);
      setNotice('保存しました。新しく生成するカードから反映されます。');
    } catch (error) {
      setError(error instanceof Error ? error.message : String(error));
      setNeedsLogin(error instanceof ApiError && error.status === 401);
    } finally {
      setPending(false);
    }
  }

  return (
    <section id="goal-editor">
      {error && (
        <p role="alert" className="admin-error">
          {error}
        </p>
      )}
      {needsLogin && <a href="/api/admin/login">Discordで再ログイン</a>}
      {notice && (
        <p role="status" className="admin-success">
          {notice}
        </p>
      )}
      {!saved && !error && <p>お題を読み込んでいます…</p>}
      {saved && (
        <>
          <div className="admin-toolbar">
            <p>
              {drafts.length}件（抽選対象 {drafts.filter((goal) => !goal.disabledReason).length}件）
              {dirty ? ' · 未保存の変更あり' : ''}
            </p>
            <button type="button" onClick={add} disabled={pending}>
              お題を追加
            </button>
            <button
              type="button"
              className="primary"
              onClick={save}
              disabled={!dirty || pending || errors.length > 0 || needsLogin}
            >
              {pending ? '保存中…' : '変更を保存'}
            </button>
          </div>
          <p className="muted">
            保存時にバックアップを作成します。作成済みルームのカードには影響しません。
          </p>
          {dirty && errors.length > 0 && (
            <ul className="admin-error" role="alert">
              {errors.map((error, index) => (
                <li key={index}>{error}</li>
              ))}
            </ul>
          )}
          <RouteAreaEditor
            areas={routeAreas}

            pending={pending}
            onChange={(areas) => {
              setRouteAreas(areas);
              setNotice('');
            }}
          />
          <BowserRouteEditor
            routes={bowserRoutes}
            areas={routeAreas}
            pending={pending}
            onChange={setBowserRoutes}
          />
          <div className="admin-layout">
            <aside className="panel admin-goal-list">
              <label>
                お題を検索
                <input
                  type="search"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="ワールド・コース・お題"
                />
              </label>
              <p className="muted">{filtered.length}件表示</p>
              <ul>
                {filtered.map((goal) => (
                  <li key={goal.key}>
                    <button
                      type="button"
                      aria-current={selected === goal.key ? 'true' : undefined}
                      onClick={() => setSelected(goal.key)}
                      disabled={pending}
                    >
                      <span className="muted">
                        {[goal.world, goal.level].filter(Boolean).join(' ') || '場所指定なし'}
                      </span>
                      <strong>
                        {goal.disabledReason ? '【保留】' : ''}
                        {goal.name || '新しいお題'}
                      </strong>
                    </button>
                  </li>
                ))}
              </ul>
            </aside>
            {current && (
              <fieldset className="panel admin-fields" disabled={pending}>
                <legend>お題を編集</legend>
                <label>
                  保留理由（入力すると抽選対象外）
                  <textarea
                    name="goal-disabled-reason"
                    value={current.disabledReason ?? ''}
                    onChange={(event) => change({ disabledReason: event.target.value })}
                  />
                </label>
                <label>
                  同じラインに配置しないグループ（カンマ区切り）
                  <input
                    name="goal-conflict-groups"
                    value={current.groups}
                    onChange={(event) => change({ groups: event.target.value })}
                  />
                </label>
                <p className="muted">
                  包含関係や似た課題には同じグループ名を指定します。経路を共有するだけのお題には指定しません。
                </p>
                <label>
                  お題の内容
                  <textarea
                    name="goal-name"
                    rows={3}
                    value={current.name}
                    onChange={(event) => change({ name: event.target.value })}
                  />
                </label>
                <div className="pair">
                  <label>
                    ワールド
                    <input
                      name="goal-world"
                      list="admin-worlds"
                      value={current.world}
                      onChange={(event) => change({ world: event.target.value })}
                      placeholder="ヨースターとう"
                    />
                  </label>
                  <label>
                    コース
                    <input
                      name="goal-level"
                      value={current.level}
                      onChange={(event) => change({ level: event.target.value })}
                      placeholder="コース4"
                    />
                  </label>
                </div>
                <datalist id="admin-worlds">
                  {worlds.map((world) => (
                    <option key={world} value={world} />
                  ))}
                </datalist>
                <p className="muted">場所を指定しない場合は空欄にします。</p>
                <label>
                  最低所要時間（分）
                  <input
                    name="goal-time"
                    aria-describedby="goal-time-help"
                    type="number"
                    min={1}
                    max={1440}
                    step={1}
                    value={current.timeMin}
                    onChange={(event) => change({ timeMin: Number(event.target.value) })}
                  />
                </label>
                <p id="goal-time-help" className="muted">
                  既存の値は実測前の暫定値です。ゲーム開始からこのお題を達成するまでの時間です。ステージへの到達・必要な準備を含め、失敗によるやり直し時間は含めません。
                </p>
                <label>
                  経由区間
                  <select
                    name="goal-route-add"
                    aria-describedby="goal-routes-help"
                    value=""
                    disabled={availableRoutes.length === 0}
                    onChange={(event) =>
                      change({ routes: [...current.routes, event.target.value] })
                    }
                  >
                    <option value="" disabled>
                      {availableRoutes.length
                        ? '経由区間を選んで追加'
                        : '追加できる区間がありません'}
                    </option>
                    {availableRoutes.map((area) => (
                      <option key={area.name} value={area.name}>
                        {routeSegmentLabel(area.name)}
                      </option>
                    ))}
                  </select>
                </label>
                <ul className="selected-route-areas" aria-label="選択済みの経由区間">
                  {current.routes.map((name) => (
                    <li key={name}>
                      <span>
                        {routeSegmentLabel(name)}
                        {RETIRED_ROUTE_AREAS.includes(name) ? '（旧エリア・要再設定）' : ''}
                      </span>
                      <button
                        type="button"
                        aria-label={`${routeSegmentLabel(name)}を経由区間から外す`}
                        onClick={() =>
                          change({ routes: current.routes.filter((area) => area !== name) })
                        }
                      >
                        外す
                      </button>
                    </li>
                  ))}
                </ul>
                <p id="goal-routes-help" className="muted">
                  共通区間と、実際に進む分岐の区間を選びます。両方の分岐を通るお題では両方を追加してください。
                </p>
                <div className="pair">
                  <label>
                    操作難度
                    <select
                      name="goal-exec"
                      value={current.exec}
                      onChange={(event) => change({ exec: Number(event.target.value) })}
                    >
                      <option value={1}>1 · 低</option>
                      <option value={2}>2 · 中</option>
                      <option value={3}>3 · 高</option>
                    </select>
                  </label>
                  <label>
                    リスク
                    <select
                      name="goal-risk"
                      value={current.risk}
                      onChange={(event) => change({ risk: Number(event.target.value) })}
                    >
                      <option value={1}>1 · 低</option>
                      <option value={2}>2 · 中</option>
                      <option value={3}>3 · 高</option>
                    </select>
                  </label>
                </div>
                <details>
                  <summary>配置バランスのタグ</summary>
                  <label>
                    エリアタグ
                    <input
                      name="goal-areas"
                      value={current.areas}
                      onChange={(event) => change({ areas: event.target.value })}
                      placeholder="yoster"
                    />
                  </label>
                  <label>
                    種類タグ
                    <input
                      name="goal-kinds"
                      value={current.kinds}
                      onChange={(event) => change({ kinds: event.target.value })}
                      placeholder="clear, coin"
                    />
                  </label>
                  <p className="muted">
                    各1〜2個。複数の場合はカンマで区切ります。area:・type: は保存時に付けます。
                  </p>
                </details>
                <button type="button" onClick={remove}>
                  このお題を削除
                </button>
              </fieldset>
            )}
          </div>
        </>
      )}
    </section>
  );
}
