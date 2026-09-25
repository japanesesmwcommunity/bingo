import { FOREST_SEGMENTS } from '../route-config';
import { forestPathsTo, replaceForestPath } from '../forest-routes';

const targets = [...new Set(FOREST_SEGMENTS.map((edge) => edge.to))];
const paths = targets.flatMap(forestPathsTo);

export function ForestRoutePicker({
  routes,
  onChange,
}: {
  routes: string[];
  onChange: (routes: string[]) => void;
}) {
  return (
    <details className="forest-route-picker">
      <summary>まよいのもりの到達経路を設定</summary>
      <label>
        森1からの到達経路
        <select
          name="forest-path"
          value=""
          onChange={(event) => {
            const path = paths[Number(event.target.value)];
            if (path) onChange(replaceForestPath(routes, path));
          }}
        >
          <option value="" disabled>
            行き先と通るゴールを選択
          </option>
          {targets.map((target) => (
            <optgroup key={target} label={`${target}に到達`}>
              {paths.map(
                (path, index) =>
                  path.target === target && (
                    <option key={index} value={index}>
                      {path.label}
                    </option>
                  ),
              )}
            </optgroup>
          ))}
        </select>
      </label>
      <p className="muted">
        選ぶと森の道中設定を置き換えます。目的地のコース自体の攻略と、森1までの道中は含みません。
        開通済みの道の移動・装備準備は、お題の所要時間に残してください。
        選んだ経路で測った所要時間を使い、道中の各ゴールの基準時間も設定してください。
      </p>
    </details>
  );
}
