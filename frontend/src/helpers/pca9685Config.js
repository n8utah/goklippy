// PCA9685 servo boards on the MCU's I2C bus. The backend reads a [pca9685 <name>]
// section for the board and [servo ...] sections with `pin: <name>:pwm<N>` for its
// channels (see backend/mcu/cfgtranslate/pca9685.go).

export const PCA9685_CHANNELS = 16;

export const PCA9685_DEFAULTS = {
  name: 'servo_board',
  address: '0x40',
  channels: PCA9685_CHANNELS,
  // MG995-style servos (as on the Basically sorter): 0.5–2.5 ms is 0–180°.
  minPulseUs: 500,
  maxPulseUs: 2500,
};

const BOARD_PREFIX = 'pca9685 ';

/** Board names must be one word: they're used in pin names (servo_board:pwm0). */
export function validPCA9685Name(name) {
  return /^[A-Za-z][A-Za-z0-9_]*$/.test(name);
}

/** "0x40", "64" -> 64, or NaN. */
export function parseI2CAddress(text) {
  const value = /^0x/i.test(text.trim()) ? parseInt(text.trim(), 16) : Number(text.trim());
  return Number.isInteger(value) && value >= 0 && value <= 0x7f ? value : NaN;
}

/** Config sections for a board and its first `channels` servos: [{ name, options }]. */
export function pca9685Sections({ name, address, channels, minPulseUs, maxPulseUs }) {
  const servos = Array.from({ length: channels }, (_, channel) => ({
    name: `servo ${name}_${channel}`,
    options: {
      pin: `${name}:pwm${channel}`,
      minimum_pulse_width: String(minPulseUs / 1e6),
      maximum_pulse_width: String(maxPulseUs / 1e6),
      maximum_servo_angle: '180',
    },
  }));
  return [
    {
      name: `${BOARD_PREFIX}${name}`,
      options: { i2c_bus: 'twi', i2c_address: `0x${parseI2CAddress(address).toString(16)}` },
    },
    ...servos,
  ];
}

/** PCA9685 boards in the merged config: [{ name, address, servos }]. */
export function pca9685BoardsInConfig(mergedSections = []) {
  return mergedSections
    .filter((section) => section.name.startsWith(BOARD_PREFIX))
    .map((section) => {
      const name = section.name.slice(BOARD_PREFIX.length);
      return {
        name,
        address: section.options?.i2c_address || '0x40',
        servos: mergedSections.filter(
          (other) =>
            other.name.startsWith('servo ') &&
            String(other.options?.pin || '').startsWith(`${name}:`),
        ).length,
      };
    });
}
