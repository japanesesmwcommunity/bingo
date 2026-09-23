// Stage name conventions from LEVEL_LIST.md. Other courses use a space.
export const WORLD_NAMES = [
  'ヨースターとう',
  'ドーナツへいや',
  'バニラドーム',
  'バニラだいち',
  'バターブリッジ',
  'チーズブリッジ',
  'まよいのもり',
  'チョコレーとう',
  'スターロード',
  'スペシャルコース',
  'まおうクッパのたに',
  'ソーダのみずうみ',
  'かっぱやま',
] as const;

export const WORLD_SEPARATORS: Record<string, string> = { かっぱやま: '' };
export const LEVEL_SEPARATORS: Record<string, string> = { しろ: 'の', とりで: 'の' };
