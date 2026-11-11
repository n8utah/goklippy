import { StrictMode } from 'react';
import ReactDOM from 'react-dom/client';

import '@fontsource-variable/inter';
import '@fontsource-variable/space-grotesk';
import '@fontsource-variable/jetbrains-mono';
import './styles/globals.css';

import App from './App';
import { startTheme } from './helpers/theme';

startTheme();

ReactDOM.createRoot(document.getElementById('root')).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
