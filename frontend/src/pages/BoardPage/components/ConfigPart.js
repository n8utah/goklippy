import { FileText } from 'lucide-react';
import { useEffect } from 'react';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/**
 * Board › Config file: the Klipper config as text. Load a file, edit, Apply to check and
 * use it (the Wiring tab follows), Save to write it to disk.
 */
export default function ConfigPart({ config }) {
  const { loadConfigFiles } = config;
  useEffect(() => {
    loadConfigFiles();
  }, [loadConfigFiles]);

  const edited = config.rawText !== (config.workspace?.raw || '');

  return (
    <div className="grid gap-4 lg:grid-cols-[260px_1fr]">
      <section className="rounded-xl border border-border bg-card p-3">
        <h2 className="px-2 pt-1 pb-2 font-display font-semibold">Config files</h2>
        {config.configFiles.length === 0 ? (
          <p className="px-2 text-sm text-muted-foreground">
            No config files in the config folder.
          </p>
        ) : (
          <ul className="flex flex-col">
            {config.configFiles.map((file) => {
              // The workspace has the full path; the list has names in the config folder.
              const path = (config.workspace?.path || '').replace(/\\/g, '/');
              const open = path === file || path.endsWith(`/${file}`);
              return (
                <li key={file}>
                  <button
                    type="button"
                    disabled={config.loading}
                    onClick={() => config.loadConfigFile(file)}
                    aria-current={open ? 'true' : undefined}
                    className={cn(
                      'flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-accent disabled:opacity-60',
                      open && 'bg-accent font-medium',
                    )}
                  >
                    <FileText className="size-4 shrink-0 text-muted-foreground" aria-hidden />
                    <span className="truncate font-mono">{file}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <section className="flex min-w-0 flex-col rounded-xl border border-border bg-card">
        <div className="flex flex-col gap-2 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="min-w-0 truncate text-sm">
            {config.workspace ? (
              <>
                <span className="font-mono">{config.workspace.path}</span>
                {config.dirty && <span className="text-warning"> · not saved to disk</span>}
              </>
            ) : (
              <span className="text-muted-foreground">No file loaded</span>
            )}
          </p>
          <div className="flex shrink-0 gap-2">
            <Button
              variant="outline"
              onClick={config.applyRaw}
              disabled={config.loading || !edited}
              title="Checks the text and uses it; the Wiring tab follows. Not yet saved to disk."
            >
              Apply edits
            </Button>
            <Button
              onClick={config.save}
              disabled={config.loading || !config.dirty || edited}
              title={config.dirty ? undefined : 'Nothing to save'}
            >
              Save to disk
            </Button>
          </div>
        </div>
        {config.error && (
          <p role="alert" className="border-b border-border px-4 py-2 text-sm text-destructive">
            {config.error}
          </p>
        )}
        {edited && (
          <p className="border-b border-border bg-warning-bg px-4 py-2 text-sm text-warning">
            Edited here; Apply edits before saving.
          </p>
        )}
        <label htmlFor="config-text" className="sr-only">
          Config file text
        </label>
        <textarea
          id="config-text"
          value={config.rawText}
          onChange={(e) => config.setRawText(e.target.value)}
          placeholder="Load a config file, or paste Klipper printer.cfg text here…"
          spellCheck={false}
          className="min-h-[60vh] w-full flex-1 resize-y rounded-b-xl bg-transparent p-4 font-mono text-[13px] leading-relaxed outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
      </section>
    </div>
  );
}
