import { RefreshCw } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';

/** Board › Connection: the serial ports, which have a Klipper board, and connecting. */
export default function ConnectionPart({ logic }) {
  const { mcu, ports } = logic;
  const busy = mcu.connecting || logic.scanning;

  return (
    <section className="rounded-xl border border-border bg-card">
      <div className="flex flex-col gap-3 border-b border-border px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="font-display text-lg font-semibold">Serial ports</h2>
          <p className="text-sm text-muted-foreground">
            {logic.scannedAt
              ? `Last checked ${new Date(logic.scannedAt).toLocaleString()}.`
              : 'Not checked for boards yet.'}{' '}
            Checking opens each port, so a board may restart.
          </p>
        </div>
        <Button variant="outline" onClick={logic.scan} disabled={busy} className="shrink-0">
          <RefreshCw className={logic.scanning ? 'animate-spin' : ''} />
          {logic.scanning ? 'Checking…' : 'Check for boards'}
        </Button>
      </div>

      {logic.portsError && (
        <p role="alert" className="border-b border-border px-5 py-3 text-sm text-warning">
          {logic.portsError}
        </p>
      )}

      {ports.length === 0 ? (
        <p className="px-5 py-6 text-sm text-muted-foreground">
          No serial ports. Plug the board in by USB, then check for boards.
        </p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="pl-5">Port</TableHead>
              <TableHead>Board</TableHead>
              <TableHead className="hidden md:table-cell">Notes</TableHead>
              <TableHead className="pr-5 text-right">
                <span className="sr-only">Connect</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {ports.map((port) => {
              const here = mcu.session?.port === port.name;
              return (
                <TableRow key={port.name}>
                  <TableCell className="pl-5 font-mono">
                    {port.name}
                    {port.name === logic.lastPort && !here && (
                      <span className="ml-2 font-sans text-xs text-muted-foreground">
                        last used
                      </span>
                    )}
                  </TableCell>
                  <TableCell>
                    {port.is_klipper ? (
                      <Badge variant="running">Klipper {port.version}</Badge>
                    ) : (
                      <span className="text-sm text-muted-foreground">
                        {port.is_usb ? 'Not checked or no answer' : 'No board'}
                      </span>
                    )}
                  </TableCell>
                  <TableCell className="hidden max-w-[28ch] truncate text-sm text-muted-foreground md:table-cell">
                    {here ? 'Connected now' : port.error || port.description || '—'}
                  </TableCell>
                  <TableCell className="pr-5 text-right">
                    {here ? (
                      <Badge variant="running">Connected</Badge>
                    ) : (
                      <Button
                        size="sm"
                        variant={port.is_klipper ? 'default' : 'outline'}
                        disabled={busy || mcu.connected}
                        title={mcu.connected ? 'Disconnect the current board first' : undefined}
                        onClick={() => logic.connect(port.name)}
                      >
                        Connect
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
    </section>
  );
}
