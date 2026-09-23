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
    <fieldset className="panel admin-fields" disabled={pending}>
      <legend>クッパへの経路</legend>
      <p className="muted">
        開始からクッパ撃破までの実測値を登録します。お題と共有する解放区間は1回分だけ計上します。未登録の場合は従来の固定時間を加算する暫定見積もりです。装備・移動順・再入場の整合性は別途確認してください。
      </p>
      {routes.map((route, index) => (
        <div key={index}>
          <label>
            経路名
            <input
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
            共有できる解放区間
            <select
              multiple
              value={route.routeAreas ?? []}
              onChange={(e) => {
                const selected = Array.from(e.target.selectedOptions, (o) => o.value);
                onChange(routes.map((r, i) => (i === index ? { ...r, routeAreas: selected } : r)));
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
        クッパ経路を追加
      </button>
    </fieldset>
  );
}
