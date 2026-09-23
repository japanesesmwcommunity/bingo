import { useEffect, useState } from 'react';
import logo from '../../logo.png';
import { api } from '../api';
import { ADMIN_LOGIN_ERRORS } from '../admin-config';
import type { AdminStatus } from '../admin-types';
import { GoalEditor } from './GoalEditor';

export function AdminPage() {
  const [status, setStatus] = useState<AdminStatus | null>(null);
  const [error, setError] = useState(
    ADMIN_LOGIN_ERRORS[new URLSearchParams(location.search).get('error') ?? ''] ?? '',
  );
  const [pending, setPending] = useState(false);
  useEffect(() => {
    let active = true;
    void api<AdminStatus>('/api/admin/session')
      .then((data) => {
        if (active) setStatus(data);
      })
      .catch((error) => {
        if (active) setError(String(error));
      });
    return () => {
      active = false;
    };
  }, []);

  async function logout() {
    setPending(true);
    try {
      await api('/api/admin/logout', 'POST', {});
      location.assign('/admin');
    } catch (error) {
      setError(String(error));
      setPending(false);
    }
  }

  return (
    <>
      <header>
        <a href="/" className="brand">
          <img src={logo} alt="SMW Bingo" width="1024" height="242" />
        </a>
        <span className="badge">お題管理</span>
      </header>
      <main id="admin-page">
        <div className="admin-heading">
          <h1>お題管理</h1>
          {status?.user && (
            <p>
              {status.user.username}{' '}
              <button type="button" onClick={logout} disabled={pending}>
                ログアウト
              </button>
            </p>
          )}
        </div>
        {error && (
          <p role="alert" className="admin-error">
            {error}
          </p>
        )}
        {!status && !error && <p>読み込んでいます…</p>}
        {status && !status.user && (
          <section className="panel admin-login">
            <h2>管理者ログイン</h2>
            <p>許可されたDiscordユーザーのみ編集できます。</p>
            {status.configured ? (
              <a className="room-link" href="/api/admin/login">
                Discordでログイン
              </a>
            ) : (
              <p>管理者ログインは未設定です。サーバーのDiscord認証設定を追加してください。</p>
            )}
          </section>
        )}
        {status?.user && <GoalEditor />}
      </main>
    </>
  );
}
