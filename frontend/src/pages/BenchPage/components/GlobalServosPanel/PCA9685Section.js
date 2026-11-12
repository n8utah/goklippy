import { Plus } from 'lucide-react';
import { useState } from 'react';

import NumberField from '@/components/NumberField/NumberField';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  parseI2CAddress,
  PCA9685_CHANNELS,
  PCA9685_DEFAULTS,
  validPCA9685Name,
} from '@/helpers/pca9685Config';

import { BenchCard } from '../BenchUi';

const STATES = {
  ok: { label: 'Working', variant: 'running' },
  missing: { label: 'Not found', variant: 'destructive' },
  error: { label: 'Error', variant: 'destructive' },
  checking: { label: 'Checking', variant: 'secondary' },
  idle: { label: 'Not checked', variant: 'idle' },
};

/** A check result (or its absence) as a badge and a message. */
function boardStatus(result, connected) {
  if (!connected)
    return { state: 'idle', label: 'Offline', message: 'Connect the board to check it.' };
  if (!result) {
    // In the config, but added after connecting.
    return {
      state: 'idle',
      label: STATES.idle.label,
      message: 'Not in this session yet: disconnect and connect again.',
    };
  }
  const state = STATES[result.state] ? result.state : 'error';
  return { state, label: STATES[state].label, message: result.message || '' };
}

function formError(form) {
  if (!validPCA9685Name(form.name)) return 'Name: one word of letters, numbers and _.';
  if (Number.isNaN(parseI2CAddress(form.address)))
    return 'Address: 0x00 to 0x7f (the board’s default is 0x40).';
  if (!Number.isInteger(form.channels) || form.channels < 1 || form.channels > PCA9685_CHANNELS)
    return `Servos: 1 to ${PCA9685_CHANNELS}.`;
  if (!(form.minPulseUs > 0 && form.maxPulseUs > form.minPulseUs && form.maxPulseUs < 20000))
    return 'Pulse: the 0° pulse above 0 and below the 180° one, which is below 20000 µs.';
  return '';
}

/**
 * PCA9685 servo boards on the RAMPS I2C pins (SCL D21, SDA D20). Adding one writes a
 * [pca9685] section and a [servo] per channel to the config; they're servos after
 * reconnecting.
 */
export default function PCA9685Section({ boards, connected, busy, onAdd, status = {}, onCheck }) {
  const [form, setForm] = useState(null);
  const [added, setAdded] = useState('');
  const checking = Object.values(status).some((s) => s.state === 'checking');
  const error = form ? formError(form) : '';
  const set = (key) => (v) => setForm((prev) => ({ ...prev, [key]: v }));

  const submit = async () => {
    if (error) return;
    if (await onAdd(form)) {
      setAdded(form.name);
      setForm(null);
    }
  };

  return (
    <BenchCard
      className="xl:col-span-2"
      title="PCA9685 servo boards"
      about="Up to 16 more servos each, on the RAMPS I2C pins."
      action={
        <div className="flex gap-2">
          {connected && boards.length > 0 && (
            <Button variant="outline" size="sm" disabled={checking} onClick={onCheck}>
              {checking ? 'Checking…' : 'Check boards'}
            </Button>
          )}
          {!form && (
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => {
                setAdded('');
                setForm({
                  ...PCA9685_DEFAULTS,
                  name: boards.length ? `servo_board${boards.length + 1}` : PCA9685_DEFAULTS.name,
                });
              }}
            >
              <Plus /> Add board
            </Button>
          )}
        </div>
      }
    >
      {boards.length === 0 && !form && (
        <p className="text-sm text-muted-foreground">None in the config.</p>
      )}
      {boards.length > 0 && (
        <ul className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {boards.map((board) => {
            const { state, label, message } = boardStatus(status[board.name], connected);
            const bad = state === 'missing' || state === 'error';
            return (
              <li
                key={board.name}
                className="flex flex-col gap-1 rounded-lg border border-border p-4"
              >
                <p className="flex flex-wrap items-center gap-2 font-medium">
                  <span className="font-mono">{board.name}</span>
                  <Badge variant={STATES[state].variant}>{label}</Badge>
                </p>
                <p className="text-sm text-muted-foreground">
                  I2C <span className="font-mono">{board.address}</span> · {board.servos} servo
                  {board.servos === 1 ? '' : 's'}
                </p>
                {message && (
                  <p className={bad ? 'text-sm text-destructive' : 'text-sm text-muted-foreground'}>
                    {message}
                  </p>
                )}
              </li>
            );
          })}
        </ul>
      )}

      {added && (
        <p className="rounded-md bg-running-bg px-3 py-2 text-sm text-running-text">
          Added {added} to the config. {connected ? 'Disconnect and connect again' : 'Connect'} to
          use its servos.
        </p>
      )}

      {form && (
        <div className="flex flex-col gap-3 rounded-lg border border-primary/40 bg-accent/40 p-4">
          <p className="text-sm text-muted-foreground">
            Wire SDA to D20, SCL to D21, VCC to 5 V and GND; the servos get their own supply on V+.
          </p>
          <div className="flex flex-wrap items-end gap-3">
            <label htmlFor="pca-name" className="flex flex-col gap-1.5 text-sm font-medium">
              Name
              <Input
                id="pca-name"
                className="w-44 font-mono"
                value={form.name}
                onChange={(e) => set('name')(e.target.value)}
              />
            </label>
            <label htmlFor="pca-address" className="flex flex-col gap-1.5 text-sm font-medium">
              I2C address
              <Input
                id="pca-address"
                className="w-28 font-mono"
                value={form.address}
                onChange={(e) => set('address')(e.target.value)}
              />
            </label>
            <NumberField
              id="pca-channels"
              label="Servos (from channel 0)"
              value={form.channels}
              min={1}
              max={PCA9685_CHANNELS}
              inputClassName="w-20"
              onChange={set('channels')}
            />
            <NumberField
              id="pca-min"
              label="Pulse at 0°"
              value={form.minPulseUs}
              min={1}
              unit="µs"
              inputClassName="w-24"
              onChange={set('minPulseUs')}
            />
            <NumberField
              id="pca-max"
              label="Pulse at 180°"
              value={form.maxPulseUs}
              min={1}
              unit="µs"
              inputClassName="w-24"
              onChange={set('maxPulseUs')}
            />
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
          <div className="flex gap-2">
            <Button disabled={busy || Boolean(error)} onClick={submit}>
              {busy ? 'Adding…' : 'Add to the config'}
            </Button>
            <Button variant="ghost" disabled={busy} onClick={() => setForm(null)}>
              Cancel
            </Button>
          </div>
        </div>
      )}
    </BenchCard>
  );
}
