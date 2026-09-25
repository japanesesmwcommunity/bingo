import type { DraftRouteArea } from '../admin-types';
import {
  FOREST_CONNECTIONS,
  FOREST_SEGMENTS,
  RETIRED_ROUTE_AREAS,
  ROUTE_SEGMENT_DESCRIPTIONS,
  routeSegmentLabel,
} from '../route-config';

interface RouteAreaEditorProps {
  areas: DraftRouteArea[];
  pending: boolean;
  onChange: (areas: DraftRouteArea[]) => void;
}

export function RouteAreaEditor({ areas, pending, onChange }: RouteAreaEditorProps) {
  const forestIDs = new Set(FOREST_SEGMENTS.map((segment) => segment.id));
  const active = areas.filter(
    (area) => !RETIRED_ROUTE_AREAS.includes(area.name) && !forestIDs.has(area.name),
  );
  const forest = areas.filter((area) => forestIDs.has(area.name));
  const missing = active.filter((area) => area.timeMin === '').length;
  return (
    <details className="panel route-area-editor">
      <summary>
        道中の重複を補正する（任意・{active.length}件・未設定{missing}件）
      </summary>
      <p className="muted">
        同じコース攻略を複数のお題で共有するときだけ設定します。記載した全コース・ゴールを攻略する時間を測り、途中までのお題には選ばないでください。マップ上の移動だけを別項目にはしません。未計測なら空欄のままで構いません。
      </p>
      <details className="forest-connections">
        <summary>まよいのもり：ゴールごとの行き先</summary>
        <p className="muted">
          森の到達経路をお題に設定すると、同じゴールの攻略時間だけを重複補正します。通常ゴールと隠しゴールは別に数えます。
          表はゴールして開く道を示します。到達しただけでは、その先の道は開きません。
        </p>
        <table>
          <thead>
            <tr>
              <th scope="col">コース</th>
              <th scope="col">通常ゴール・クリア</th>
              <th scope="col">隠しゴール</th>
            </tr>
          </thead>
          <tbody>
            {FOREST_CONNECTIONS.map((connection) => (
              <tr key={connection.course}>
                <th scope="row">{connection.course}</th>
                <td>{connection.normal}</td>
                <td>{connection.secret ?? 'なし'}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="muted">
          城5へは森3の隠しゴールから直結します。とりでへは森4の隠しゴールに加えて、ひみつのコースの攻略が必要です。
          おばけやしきは森1の隠しゴールからも行けるため、森2・3を経由するとは限りません。
          通常ゴールだけでは森2→3→おばけやしき→4→2と循環します。
        </p>
      </details>
      <details className="forest-times">
        <summary>まよいのもり：ゴール別の攻略時間</summary>
        <p className="muted">
          入場から各ゴールまでの実測分数です。森への到達時間やマップ上の移動時間は含めません。使わないゴールは空欄のままで構いません。
        </p>
        <fieldset disabled={pending}>
          <legend className="muted">通常・隠しゴールを個別に計測</legend>
          {forest.map((area) => (
            <label key={area.key}>
              {routeSegmentLabel(area.name)}（分）
              <input
                name="forest-goal-time"
                aria-label={`${routeSegmentLabel(area.name)}の基準時間（分）`}
                type="number"
                min={1}
                max={1440}
                step={1}
                placeholder="未設定"
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
          ))}
        </fieldset>
      </details>
      <fieldset disabled={pending}>
        <legend className="muted">区間と基準時間</legend>
        {active.map((area) => (
          <div className="route-area-row" key={area.key}>
            <div className="route-area-name">
              <span>{routeSegmentLabel(area.name)}</span>
              {Object.hasOwn(ROUTE_SEGMENT_DESCRIPTIONS, area.name) && (
                <p className="muted">{ROUTE_SEGMENT_DESCRIPTIONS[area.name]}</p>
              )}
            </div>
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
