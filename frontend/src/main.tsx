import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { AdminPage } from './components/AdminPage';
import '../style.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    {location.pathname === '/admin' ? (
      <AdminPage />
    ) : (
      <App cardOnly={new URLSearchParams(location.search).get('view') === 'card'} />
    )}
  </StrictMode>,
);
