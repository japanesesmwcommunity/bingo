import type { FinishRoute } from '../types';
import type { DraftRouteArea } from '../admin-types';
import { RETIRED_ROUTE_AREAS, routeSegmentLabel } from '../route-config';

export function BowserRouteEditor({
  routes,
  areas,
  pending,
  onChange,
}: {
  routes: FinishRoute[];
  areas: DraftRouteArea[];
  pending: boolean;
  onChange: (routes: FinishRoute[]) => void;
}) {
  return (
    <details className="panel bowser-time-editor" open={routes.length > 0}>
      <summary>クッパ撃破までの所要時間（任意）</summary>
      <fieldset className="admin-fields" disabled={pending}>
        <legend>ゲーム開始から撃破までの実測値</legend>
        <p className="muted">
          「1ライン＋クッパ撃破」の見積もりに使います。未登録なら従来の固定時間（通常15分）をお題の見積もりに加算します。「1ラインのみ」には加算しません。
        </p>
        <p className="muted">
          登録するのは新規開始から撃破までの合計時間です。現在地からの残り時間ではありません。お題と共通する攻略済みのまとまりだけを差し引きます。城内の攻略・クッパ戦・再移動の時間は残してください。
        </p>
        <p className="muted">
          例：ドーナツのひみつのおばけやしき→ネイティブスター1〜4の隠しゴール→城の正面。裏口は谷2の隠しゴールから谷のとりでをクリアして解放します。経路名だけで道中や時間が自動設定されることはありません。
        </p>
        {routes.map((route, index) => (
          <div key={index}>
            <label>
              経路名
              <input
                placeholder="例：スターロード経由・正面"
                aria-label={`クッパ経路${index + 1}の名前`}
                value={route.name}
                onChange={(e) =>
                  onChange(routes.map((r, i) => (i === index ? { ...r, name: e.target.value } : r)))
                }
              />
            </label>
            <label>
              開始から撃破まで（分）
              <input
                type="number"
                min={1}
                max={1440}
                value={route.timeMin}
                onChange={(e) =>
                  onChange(
                    routes.map((r, i) =>
                      i === index ? { ...r, timeMin: Number(e.target.value) } : r,
                    ),
                  )
                }
              />
            </label>
            <label>
              全コース・ゴールの攻略を共有するまとまり
              <select
                multiple
                value={route.routeAreas ?? []}
                onChange={(e) => {
                  const selected = Array.from(e.target.selectedOptions, (o) => o.value);
                  onChange(
                    routes.map((r, i) => (i === index ? { ...r, routeAreas: selected } : r)),
                  );
                }}
              >
                {areas
                  .filter((a) => a.timeMin !== '' && !RETIRED_ROUTE_AREAS.includes(a.name))
                  .map((a) => (
                    <option key={a.name} value={a.name}>
                      {routeSegmentLabel(a.name)}
                    </option>
                  ))}
              </select>
            </label>
            <button type="button" onClick={() => onChange(routes.filter((_, i) => i !== index))}>
              この経路を外す
            </button>
          </div>
        ))}
        <button type="button" onClick={() => onChange([...routes, { name: '', timeMin: 1 }])}>
          実測した行き方を追加
        </button>
      </fieldset>
    </details>
  );
}
