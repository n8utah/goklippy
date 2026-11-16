import { Trash2 } from 'lucide-react';
import { useState } from 'react';

import Segmented from '@/components/Segmented/Segmented';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';

import { SPLIT_OPTIONS } from '../RuleEditorPage.logic';
import MultiPicker from './MultiPicker';
import SetPicker from './SetPicker';

const KIND_OPTIONS = [
  { value: 'pieces', label: 'Pieces' },
  { value: 'sets', label: 'Sets' },
];

function Field({ label, hint, htmlFor, children }) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={htmlFor}>{label}</Label>
      {hint && <p className="-mt-1 text-sm text-muted-foreground">{hint}</p>}
      {children}
    </div>
  );
}

/** Part numbers typed as a list ("3001, 3003 3004"); saved when the field is left. */
function PartsField({ parts, onChange, disabled }) {
  const [text, setText] = useState((parts ?? []).join(', '));
  return (
    <Input
      id="rule-parts"
      value={text}
      disabled={disabled}
      placeholder="Any part number"
      className="font-mono"
      onChange={(e) => setText(e.target.value)}
      onBlur={() => onChange(text.split(/[\s,;]+/).filter(Boolean))}
    />
  );
}

/** One rule: its name, what it takes, and how its pieces split into bin groups. */
export default function RuleForm({ rule, options, readOnly, onChange, onRemove }) {
  // The form is keyed by the rule, so this is read once per rule.
  const kind = rule.kind === 'sets' || rule.sets?.length ? 'sets' : 'pieces';

  const families = options?.families ?? [];
  const types = options?.types ?? [];
  const sizes = (options?.sizes ?? []).map((s) => ({ ...s }));
  const colors = options?.colors ?? [];

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-end gap-4">
        <div className="min-w-48 flex-1">
          <Field label="Name" htmlFor="rule-name">
            <Input
              id="rule-name"
              value={rule.name}
              disabled={readOnly}
              onChange={(e) => onChange({ name: e.target.value })}
            />
          </Field>
        </div>
        <label className="flex items-center gap-2 pb-2 text-sm">
          <Switch
            checked={!rule.disabled}
            disabled={readOnly}
            onCheckedChange={(on) => onChange({ disabled: !on })}
          />
          On
        </label>
        {!readOnly && (
          <Button variant="ghost" className="text-destructive" onClick={onRemove}>
            <Trash2 /> Delete rule
          </Button>
        )}
      </div>
      <Field
        label="Description"
        hint="Shown on the viewer site beside the rule’s name."
        htmlFor="rule-description"
      >
        <Input
          id="rule-description"
          value={rule.description ?? ''}
          disabled={readOnly}
          onChange={(e) => onChange({ description: e.target.value })}
        />
      </Field>

      <Field label="Takes">
        <Segmented
          label="What the rule takes"
          value={kind}
          options={KIND_OPTIONS}
          disabled={readOnly}
          onChange={(k) => {
            onChange(k === 'sets' ? { kind: 'sets' } : { kind: '', sets: [] });
          }}
        />
      </Field>

      {kind === 'sets' ? (
        <Field
          label="Sets"
          hint="Only the parts these sets still need, each set into its own bins; once a set has enough of a part, the rest go on to the next rules."
        >
          <SetPicker
            sets={rule.sets ?? []}
            disabled={readOnly}
            onChange={(s) => onChange({ sets: s })}
          />
        </Field>
      ) : (
        <>
          <p className="-mt-2 text-sm text-muted-foreground">
            A piece must match everything filled in; leave a field empty for “any”.
          </p>
          <Field
            label="Part type families"
            hint="A whole family: “Plate” takes every plate, round and modified ones too."
          >
            <MultiPicker
              label="families"
              options={families}
              value={rule.families}
              disabled={readOnly}
              onChange={(v) => onChange({ families: v })}
            />
          </Field>
          <Field
            label="Exact part types"
            hint="“Brick (standard)” is only the plain brick; “Brick, Round” only round ones."
          >
            <MultiPicker
              label="types"
              options={types}
              value={rule.types}
              grouped
              disabled={readOnly}
              onChange={(v) => onChange({ types: v })}
            />
          </Field>
          <Field label="Sizes" hint="From the part’s name. 2 x 4 and 4 x 2 are the same size.">
            <MultiPicker
              label="sizes"
              options={sizes}
              value={rule.sizes}
              allowCustom
              placeholder="Search or type, like 2 x 14"
              disabled={readOnly}
              onChange={(v) => onChange({ sizes: v })}
            />
          </Field>
          <Field
            label="Part numbers"
            hint="BrickLink numbers; printed and assembled variants count as their base part."
            htmlFor="rule-parts"
          >
            <PartsField
              key={(rule.parts ?? []).join(',')}
              parts={rule.parts}
              disabled={readOnly}
              onChange={(v) => onChange({ parts: v })}
            />
          </Field>
          <Field label="Colours">
            <MultiPicker
              label="colours"
              options={colors}
              value={rule.colors}
              disabled={readOnly}
              onChange={(v) => onChange({ colors: v })}
            />
          </Field>
          <Field
            label="Bin groups"
            hint="One group for everything it takes, or a group per size, part type or colour (made as pieces arrive)."
          >
            <Segmented
              label="Split into bin groups"
              value={rule.split ?? ''}
              options={SPLIT_OPTIONS}
              disabled={readOnly}
              onChange={(v) => onChange({ split: v })}
              className="flex-wrap"
            />
          </Field>
        </>
      )}
    </div>
  );
}
