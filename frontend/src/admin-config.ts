import type { Goal } from './types';

export const NEW_GOAL: Goal = {
  name: '',
  world: '',
  level: '',
  timeMin: 5,
  exec: 1,
  risk: 1,
  tags: ['area:multi', 'type:clear'],
};

export const ADMIN_LOGIN_ERRORS: Record<string, string> = {
  state: 'ログインの有効期限が切れたか、別のブラウザで開かれました。もう一度ログインしてください。',
  cancelled: 'Discordログインがキャンセルされました。',
  discord: 'Discordで本人を確認できませんでした。もう一度ログインしてください。',
  denied: 'このDiscordユーザーには管理権限がありません。',
  busy: '現在ログインできません。少し待ってから再試行してください。',
};
