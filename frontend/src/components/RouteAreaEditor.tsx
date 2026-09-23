import type { DraftRouteArea } from '../admin-types';
import { RETIRED_ROUTE_AREAS, routeSegmentLabel } from '../route-config';

interface RouteAreaEditorProps {
  areas: DraftRouteArea[];
  pending: boolean;
  onChange: (areas: DraftRouteArea[]) => void;
}

export function RouteAreaEditor({ areas, pending, onChange }: RouteAreaEditorProps) {
  const active = areas.filter((area) => !RETIRED_ROUTE_AREAS.includes(area.name));
  const legacy = areas.filter((area) => RETIRED_ROUTE_AREAS.includes(area.name));
  const missing = active.filter((area) => area.timeMin === '').length;
  return (
    <details className="panel route-area-editor" open>
      <summary>
        全区間の基準時間（{active.length}件・未設定{missing}件）
      </summary>
      <p className="muted">
        各区間だけの解放時間を入力します。バニラ1は分岐前までを共通とし、出口攻略・再入場を別計上します。使用しない区間は未設定のまま保存できます。
      </p>
      {legacy.length > 0 && (
        <div className="legacy-route-times">
          <p>分割前の参考値（計算の重複補正には使用しません）</p>
          <ul>
            {legacy.map((area) => (
              <li key={area.key}>
                {area.name}：{area.timeMin === '' ? '未設定' : `${area.timeMin}分`}
              </li>
            ))}
          </ul>
          <p className="muted">
            元の時間から各分岐の時間は決められないため、新しい区間は未設定です。お題に残っている旧エリアも、実際に通る区間へ選び直してください。
          </p>
        </div>
      )}
      <fieldset disabled={pending}>
        <legend className="muted">区間と基準時間</legend>
        {active.map((area) => (
          <div className="route-area-row" key={area.key}>
            <span className="route-area-name">{routeSegmentLabel(area.name)}</span>
            <label>
              基準時間（分）
              <input
                name="route-area-time"
                type="number"
                min={1}
                max={1440}
                step={1}
                placeholder="未設定"
                aria-label={`${routeSegmentLabel(area.name)}の基準時間（分）`}
                aria-invalid={area.timeMin !== '' && (area.timeMin < 1 || area.timeMin > 1440)}
                value={area.timeMin}
                onChange={(event) =>
                  onChange(
                    areas.map((item) =>
                      item.key === area.key
                        ? {
                            ...item,
                            timeMin: event.target.value === '' ? '' : Number(event.target.value),
                          }
                        : item,
                    ),
                  )
                }
              />
            </label>
          </div>
        ))}
      </fieldset>
    </details>
  );
}
