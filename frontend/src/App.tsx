import logo from '../logo.png';
import { GameRoom } from './components/GameRoom';
import { Lobby } from './components/Lobby';
import { useRoom } from './use-room';
import { PlayerCard } from './components/PlayerCard';

export function App({ cardOnly = false }: { cardOnly?: boolean }) {
  const controller = useRoom();
  if (cardOnly) return <PlayerCard controller={controller} />;
  return (
    <>
      <header>
        <a href="/" className="brand">
          <img src={logo} alt="SMW Bingo" width="1024" height="242" />
        </a>
        <span className="badge">PROTOTYPE · 最大4人</span>
      </header>
      <main>
        <div id="notice" role="alert" hidden={!controller.notice}>
          {controller.notice}
        </div>
        <Lobby
          create={controller.create}
          join={controller.join}
          pending={controller.pending}
          inviteRoomId={controller.inviteRoomId}
          lobbyView={controller.lobbyView}
          hidden={Boolean(controller.snapshot)}
        />
        {controller.snapshot && <GameRoom snapshot={controller.snapshot} controller={controller} />}
      </main>
      <footer>
        SMW Bingo Beta · <a href="/admin">お題管理</a>
      </footer>
    </>
  );
}
