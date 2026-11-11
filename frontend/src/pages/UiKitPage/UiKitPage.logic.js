import { useState } from 'react';
import { toast } from 'sonner';

import { useTheme } from '../../helpers/theme';

// The design system's tokens, shown as swatches (styles/globals.css).
export const SWATCHES = [
  { name: 'background', className: 'bg-background' },
  { name: 'card', className: 'bg-card' },
  { name: 'muted', className: 'bg-muted' },
  { name: 'accent (hover)', className: 'bg-accent' },
  { name: 'border', className: 'bg-border' },
  { name: 'primary', className: 'bg-primary' },
  { name: 'media', className: 'bg-media' },
  { name: 'running', className: 'bg-running' },
  { name: 'warning', className: 'bg-warning' },
  { name: 'destructive', className: 'bg-destructive' },
  { name: 'idle', className: 'bg-idle' },
];

export const THEMES = [
  { id: 'light', label: 'Light' },
  { id: 'dark', label: 'Dark' },
  { id: 'system', label: 'System' },
];

export function useUiKitPageLogic() {
  const { theme, resolved, setTheme } = useTheme();
  const [exitPulse, setExitPulse] = useState(2);
  const [lightOn, setLightOn] = useState(true);
  const [searchOpen, setSearchOpen] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);

  return {
    theme,
    resolved,
    setTheme,
    exitPulse,
    setExitPulse,
    lightOn,
    setLightOn,
    searchOpen,
    setSearchOpen,
    dialogOpen,
    setDialogOpen,
    toastSaved: () =>
      toast.success('Exit pulse saved', { description: 'Applies to the running sort now.' }),
    toastWarn: () => toast.warning('Motor power is low', { description: '9.8 V, needs 10.5 V.' }),
  };
}
