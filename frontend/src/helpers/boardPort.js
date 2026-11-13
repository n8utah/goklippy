// The serial port the board was last connected on, remembered in this browser, so
// Connect (top bar, Machine › Board) can go straight back to it.

const LAST_PORT_KEY = 'goklippy_last_mcu_port';

export function lastBoardPort() {
  try {
    return localStorage.getItem(LAST_PORT_KEY);
  } catch {
    return null;
  }
}

export function rememberBoardPort(port) {
  try {
    localStorage.setItem(LAST_PORT_KEY, port);
  } catch {
    // remembered for this visit only
  }
}

/**
 * The port to connect to without asking: the last one used if it's still there, else
 * the only Klipper board found, else the only port. null when it isn't clear.
 */
export function pickBoardPort(ports, last = lastBoardPort()) {
  const names = ports.map((p) => p.name);
  if (last && names.includes(last)) return last;
  const klipper = ports.filter((p) => p.is_klipper);
  if (klipper.length === 1) return klipper[0].name;
  if (ports.length === 1) return ports[0].name;
  return null;
}
