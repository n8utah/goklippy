import { createContext, useContext } from 'react';

/** Opens the quick editor on a setting: useQuickEditor().open('feeder.exitPulseDeg'). */
export const QuickEditorContext = createContext({ open: () => {} });

export const useQuickEditor = () => useContext(QuickEditorContext);
