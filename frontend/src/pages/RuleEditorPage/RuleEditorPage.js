import { ArrowDown, ArrowLeft, ArrowUp, Copy, Save } from 'lucide-react';
import { Link, useParams } from 'react-router';

import Segmented from '@/components/Segmented/Segmented';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

import AddRuleMenu from './components/AddRuleMenu';
import RuleForm from './components/RuleForm';
import SaveBinsDialog from './components/SaveBinsDialog';
import WhereCheck from './components/WhereCheck';
import { LEFTOVER_OPTIONS, describeRule, useRuleEditorLogic } from './RuleEditorPage.logic';

function RuleList({ logic }) {
  const { doc, selectedId, select, moveRule, readOnly, colorName } = logic;
  if (doc.rules.length === 0) {
    return (
      <p className="rounded-xl border border-dashed border-border p-5 text-sm text-muted-foreground">
        {readOnly
          ? 'No rules: every piece goes where the leftovers go.'
          : 'No rules yet: every piece goes where the leftovers go. Add a premade rule and change it, or start from a blank one.'}
      </p>
    );
  }
  return (
    <ol className="flex flex-col gap-2" aria-label="Rules, first match wins">
      {doc.rules.map((r, i) => (
        <li key={r.id}>
          <div
            className={cn(
              'flex items-start gap-2 rounded-xl border bg-card p-3',
              r.id === selectedId ? 'border-primary' : 'border-border',
              r.disabled && 'opacity-60',
            )}
          >
            <button
              type="button"
              className="min-w-0 flex-1 text-left"
              onClick={() => select(r.id)}
              aria-current={r.id === selectedId}
            >
              <span className="flex flex-wrap items-center gap-2">
                <span className="font-mono text-xs text-muted-foreground">{i + 1}</span>
                <span className="font-medium">{r.name}</span>
                {r.disabled && <Badge variant="outline">Off</Badge>}
              </span>
              <span className="mt-0.5 block text-sm text-muted-foreground">
                {describeRule(r, colorName)}
              </span>
            </button>
            {!readOnly && (
              <span className="flex shrink-0 flex-col">
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  aria-label={`Move ${r.name} up`}
                  disabled={i === 0}
                  onClick={() => moveRule(r.id, -1)}
                >
                  <ArrowUp />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  aria-label={`Move ${r.name} down`}
                  disabled={i === doc.rules.length - 1}
                  onClick={() => moveRule(r.id, 1)}
                >
                  <ArrowDown />
                </Button>
              </span>
            )}
          </div>
        </li>
      ))}
    </ol>
  );
}

/**
 * Rules › Profiles › a profile: its rules in order (first match wins), what happens to
 * the rest, and a check of where a part would go.
 */
const RuleEditorPage = () => {
  const { id } = useParams();
  const logic = useRuleEditorLogic(id);
  const { data, doc, readOnly } = logic;

  if (!doc) {
    return (
      <div className="ds flex flex-col gap-4">
        <BackLink />
        {logic.error ? (
          <p className="text-sm text-destructive">{logic.error}</p>
        ) : (
          <p className="text-sm text-muted-foreground">Loading…</p>
        )}
      </div>
    );
  }

  return (
    <div className="ds flex flex-col gap-4">
      <BackLink />
      <section className="flex flex-col gap-4 rounded-xl border border-border bg-card px-5 py-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end">
          <div className="flex min-w-0 flex-1 flex-col gap-1.5">
            <Label htmlFor="profile-name">Profile name</Label>
            <Input
              id="profile-name"
              value={doc.name}
              disabled={readOnly}
              onChange={(e) => logic.setField('name', e.target.value)}
              className="font-display text-lg font-semibold"
            />
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            {readOnly ? (
              <Button onClick={logic.copy} disabled={logic.saving}>
                <Copy /> Copy to change it
              </Button>
            ) : (
              <>
                <Button variant="outline" onClick={logic.copy} disabled={logic.saving}>
                  <Copy /> Copy
                </Button>
                <Button onClick={() => logic.save()} disabled={logic.saving || !logic.dirty}>
                  <Save /> {logic.saving ? 'Saving…' : logic.dirty ? 'Save' : 'Saved'}
                </Button>
              </>
            )}
          </div>
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="profile-description">Description</Label>
          <Input
            id="profile-description"
            value={doc.description ?? ''}
            disabled={readOnly}
            onChange={(e) => logic.setField('description', e.target.value)}
          />
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-sm font-medium">Pieces no rule takes go</span>
          <Segmented
            label="Pieces no rule takes"
            value={doc.leftover}
            options={LEFTOVER_OPTIONS.map((o) =>
              o.value === 'misc' ? { ...o, label: 'To misc' } : o,
            )}
            disabled={readOnly}
            onChange={(v) => logic.setField('leftover', v)}
          />
        </div>
        {readOnly && (
          <p className="text-sm text-muted-foreground">
            Built-in profiles can’t be changed. Copy it to make one you can.
          </p>
        )}
        {data.converted && (
          <p className="text-sm text-muted-foreground">
            Uploaded from Hive. Saving changes it in place into a profile made here; uploading the
            file again adds a new one.
          </p>
        )}
        {data.profile.used_by?.length > 0 && (
          <p className="text-sm text-muted-foreground">
            Sorting {data.profile.used_by.map((c) => c.name).join(', ')}.
            {data.in_bins &&
              !readOnly &&
              ' Its pieces are in the bins, so saving asks what’s in them.'}
          </p>
        )}
        {logic.error && <p className="text-sm text-destructive">{logic.error}</p>}
        {logic.notice && (
          <p className="rounded-lg border border-running/40 bg-running-bg px-3 py-2 text-sm text-running-text">
            {logic.notice}
          </p>
        )}
      </section>

      <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <section className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-2">
            <h2 className="font-display text-lg font-semibold">Rules</h2>
            {!readOnly && <AddRuleMenu presets={logic.options?.presets} onAdd={logic.addRule} />}
          </div>
          <p className="text-sm text-muted-foreground">
            Each piece goes to the first rule that takes it.
          </p>
          <RuleList logic={logic} />
        </section>

        <section className="flex flex-col gap-4">
          {doc.rules.length > 0 && (
            <div className="rounded-xl border border-border bg-card px-5 py-4">
              {logic.selected ? (
                <RuleForm
                  key={logic.selected.id}
                  rule={logic.selected}
                  options={logic.options}
                  readOnly={readOnly}
                  onChange={(patch) => logic.updateRule(logic.selected.id, patch)}
                  onRemove={() => logic.removeRule(logic.selected.id)}
                />
              ) : (
                <p className="text-sm text-muted-foreground">Pick a rule to see what it takes.</p>
              )}
            </div>
          )}
          <div className="rounded-xl border border-border bg-card px-5 py-4">
            <h2 className="mb-1 font-display text-lg font-semibold">Where would this go?</h2>
            <p className="mb-3 text-sm text-muted-foreground">
              Checks the rules as they are here, saved or not.
            </p>
            <WhereCheck doc={doc} colors={logic.options?.colors ?? []} />
          </div>
        </section>
      </div>

      <SaveBinsDialog
        open={logic.askBins}
        busy={logic.saving}
        onCancel={logic.closeAskBins}
        onSave={(bins) => logic.save(bins)}
      />
    </div>
  );
};

function BackLink() {
  return (
    <Link
      to="/rules/profiles"
      className="inline-flex w-fit items-center gap-1 text-sm font-medium text-primary-text hover:underline"
    >
      <ArrowLeft className="size-4" /> All profiles
    </Link>
  );
}

export default RuleEditorPage;
