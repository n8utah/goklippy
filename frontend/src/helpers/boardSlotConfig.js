function normalizePin(pin) {
  return String(pin || '')
    .trim()
    .replace(/^[!^]+/, '');
}

function pinsMatchSlot(slot, options) {
  if (!slot?.pins || !options) {
    return false;
  }
  switch (slot.type) {
    case 'stepper':
      return (
        normalizePin(options.step_pin) === normalizePin(slot.pins.step) &&
        normalizePin(options.dir_pin) === normalizePin(slot.pins.dir)
      );
    case 'thermistor':
      return normalizePin(options.sensor_pin) === normalizePin(slot.pins.sensor);
    case 'fan':
    case 'relay':
    case 'heater':
      // [output_pin] and [fan] use pin; an older [extruder]/[heater_bed] used heater_pin.
      return normalizePin(options.pin || options.heater_pin) === normalizePin(slot.pins.pin);
    case 'servo':
      return normalizePin(options.pin) === normalizePin(slot.pins.pin);
    case 'endstop':
      return normalizePin(options.pin) === normalizePin(slot.pins.pin);
    default:
      return false;
  }
}

/**
 * The MOSFET outputs (D8, D9, D10 on RAMPS) are plain on/off IO for us, whatever the
 * board calls them: connecting one writes an [output_pin], never a heater or fan.
 */
export function isOnOffSlot(slot) {
  return ['heater', 'fan', 'relay'].includes(slot?.type);
}

/** Preferred Klipper section name for a board profile slot. */
export function configSectionName(slotId, slot) {
  switch (slot?.type) {
    case 'stepper':
      return slotId;
    case 'fan':
    case 'heater':
    case 'relay':
      return `output_pin ${slotId}`;
    case 'thermistor':
      return `temperature_sensor ${slotId.replace(/^thermistor_/, '')}`;
    case 'servo': {
      const suffix = slotId.replace(/^servo_/, '') || slotId;
      return `servo ${suffix}`;
    }
    case 'endstop':
      return slotId;
    default:
      return slotId;
  }
}

/** Build Klipper section options from a profile slot definition. */
export function configOptionsForSlot(slotId, slot, patch = {}) {
  switch (slot?.type) {
    case 'stepper':
      return {
        step_pin: slot.pins.step,
        dir_pin: slot.pins.dir,
        ...(slot.pins.enable ? { enable_pin: slot.pins.enable } : {}),
        microsteps: '16',
        rotation_distance: '40',
      };
    case 'fan':
    case 'heater':
    case 'relay':
      return { pin: slot.pins.pin, value: '0', shutdown_value: '0' };
    case 'thermistor':
      return {
        sensor_pin: slot.pins.sensor,
        ...(slot.defaults?.sensor_type ? { sensor_type: String(slot.defaults.sensor_type) } : {}),
      };
    case 'servo':
      return {
        pin: slot.pins.pin,
        maximum_servo_angle: '180',
      };
    case 'endstop':
      return { pin: slot.pins.pin };
    default:
      return patch.options || {};
  }
}

/** Find the merged config section that matches a board slot, if any. */
export function findSectionForSlot(slotId, slot, mergedSections = []) {
  const preferred = configSectionName(slotId, slot);
  const byName = mergedSections.find((section) => section.name === preferred);
  if (byName && pinsMatchSlot(slot, byName.options)) {
    return byName;
  }
  return mergedSections.find((section) => pinsMatchSlot(slot, section.options)) || null;
}

/** Derive visual slot states from the merged Klipper config + board profile. */
export function slotStatesFromWorkspace(profile, mergedSections = []) {
  const states = {};
  for (const [slotId, slot] of Object.entries(profile?.slots || {})) {
    const section = findSectionForSlot(slotId, slot, mergedSections);
    if (section) {
      states[slotId] = {
        connected: true,
        label: slot.label || slotId,
        type: slot.type,
        section: section.name,
      };
    }
  }
  return states;
}

export function applyWorkspace(ws, { setWorkspace, setRawText, setSlotStates, profile }) {
  setWorkspace(ws);
  setRawText(ws.raw || '');
  if (profile) {
    setSlotStates(slotStatesFromWorkspace(profile, ws.merged_sections || []));
  }
}
