import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
// Import order is intentional: shared shell styles load first, followed by
// authentication, profile, and motion-specific overrides.
import './styles.css';
import './extra.css';
import './auth.css';
import './profile.css';
import './motion.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)

