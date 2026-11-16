import { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';

import {
  createEditorProfile,
  getEditorProfile,
  getProfileEditorOptions,
  saveEditorProfile,
} from '../../helpers/api';

/** A rule id the backend keeps: unique, no slash. */
export function newRuleId() {
  return `rule-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

export const LEFTOVER_OPTIONS = [
  { value: 'misc', label: 'Misc' },
  { value: 'part_type', label: 'By part type' },
  { value: 'color', label: 'By colour' },
];

export const SPLIT_OPTIONS = [
  { value: '', label: 'One group' },
  { value: 'size', label: 'By size' },
  { value: 'family', label: 'By part type' },
  { value: 'type', label: 'By exact type' },
  { value: 'color', label: 'By colour' },
];

/** "2x4" -> "2 x 4". */
export function sizeLabel(size) {
  return size.replaceAll('x', ' x ');
}

/** What a rule takes, in a line. */
export function describeRule(rule, colorName) {
  if (rule.kind === 'sets' && !rule.sets?.length)
    return 'Sets: none added yet, so it takes nothing';
  if (rule.sets?.length) {
    const names = rule.sets.map((s) => s.number);
    return `${rule.sets.length === 1 ? 'Set' : 'Sets'} ${names.join(', ')}, each into its own bins until it has enough`;
  }
  const bits = [];
  const types = [
    ...(rule.families ?? []).map((f) => `every ${f.toLowerCase()}`),
    ...(rule.types ?? []).map((t) => (t.includes(',') ? t : `${t} (standard)`)),
  ];
  if (types.length) bits.push(types.join(', '));
  if (rule.sizes?.length) bits.push(rule.sizes.map(sizeLabel).join(', '));
  if (rule.parts?.length)
    bits.push(
      `${rule.parts.length} part number${rule.parts.length === 1 ? '' : 's'} (${rule.parts.slice(0, 3).join(', ')}${rule.parts.length > 3 ? ', …' : ''})`,
    );
  if (rule.colors?.length) bits.push(`in ${rule.colors.map(colorName).join(', ')}`);
  let text = bits.length ? bits.join(' · ') : 'Every piece';
  const split = SPLIT_OPTIONS.find((o) => o.value === (rule.split ?? ''));
  if (split && split.value) text += ` · a bin group ${split.label.toLowerCase()}`;
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/**
 * Rules › Profiles › one profile: its rules as a draft, saved together. `id` is the
 * profile's id from the URL.
 */
export function useRuleEditorLogic(id) {
  const navigate = useNavigate();
  const [data, setData] = useState(null); // { profile, document, read_only, converted, in_bins }
  const [doc, setDoc] = useState(null);
  const [options, setOptions] = useState(null);
  const [selectedId, setSelectedId] = useState(null);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [saving, setSaving] = useState(false);
  const [askBins, setAskBins] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setData(null);
    setDoc(null);
    setError('');
    getEditorProfile(id).then(
      (d) => {
        if (cancelled) return;
        setData(d);
        setDoc(d.document);
        setSelectedId(d.document.rules[0]?.id ?? null);
        setDirty(false);
      },
      (err) => !cancelled && setError(err.message),
    );
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    getProfileEditorOptions().then(setOptions, (err) => setError(err.message));
  }, []);

  const colorNames = useMemo(
    () => new Map((options?.colors ?? []).map((c) => [c.value, c.label])),
    [options],
  );
  const colorName = useCallback((c) => colorNames.get(c) ?? `colour ${c}`, [colorNames]);

  const change = (fn) => {
    setDoc((d) => fn(d));
    setDirty(true);
    setNotice('');
  };

  const setField = (field, value) => change((d) => ({ ...d, [field]: value }));

  const updateRule = (ruleId, patch) =>
    change((d) => ({
      ...d,
      rules: d.rules.map((r) => (r.id === ruleId ? { ...r, ...patch } : r)),
    }));

  const addRule = (rule) => {
    const added = { ...structuredClone(rule), id: newRuleId() };
    change((d) => ({ ...d, rules: [...d.rules, added] }));
    setSelectedId(added.id);
  };

  const moveRule = (ruleId, step) =>
    change((d) => {
      const rules = [...d.rules];
      const i = rules.findIndex((r) => r.id === ruleId);
      const j = i + step;
      if (i < 0 || j < 0 || j >= rules.length) return d;
      [rules[i], rules[j]] = [rules[j], rules[i]];
      return { ...d, rules };
    });

  const removeRule = (ruleId) => {
    const i = doc.rules.findIndex((r) => r.id === ruleId);
    const rest = doc.rules.filter((r) => r.id !== ruleId);
    change((d) => ({ ...d, rules: d.rules.filter((r) => r.id !== ruleId) }));
    setSelectedId(rest[Math.min(i, rest.length - 1)]?.id ?? null);
  };

  const save = async (bins = '') => {
    if (data.in_bins && !bins) {
      setAskBins(true);
      return;
    }
    setSaving(true);
    setError('');
    try {
      const d = await saveEditorProfile(id, doc, bins);
      setData(d);
      setDoc(d.document);
      setDirty(false);
      setAskBins(false);
      setNotice(
        d.in_bins
          ? 'Saved. The next piece sorts by these rules.'
          : 'Saved. Pick it for a collection on Run › Collections.',
      );
    } catch (err) {
      if (err.body?.needs_bins) setAskBins(true);
      else setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const copy = async () => {
    setSaving(true);
    setError('');
    try {
      const saved = await createEditorProfile({ copy_of: Number(id) });
      navigate(`/rules/profiles/${saved.id}`);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return {
    data,
    doc,
    options,
    colorName,
    selected: doc?.rules.find((r) => r.id === selectedId) ?? null,
    selectedId,
    select: setSelectedId,
    dirty,
    error,
    notice,
    saving,
    readOnly: Boolean(data?.read_only),
    setField,
    updateRule,
    addRule,
    moveRule,
    removeRule,
    save,
    copy,
    askBins,
    closeAskBins: () => setAskBins(false),
  };
}
