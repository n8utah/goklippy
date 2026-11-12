/**
 * A backend message as a sentence: capital first letter and a full stop. The backend's
 * errors are Go style ("no board connected; connect one under Machine › Board").
 */
export const sentence = (s) =>
  s ? `${s[0].toUpperCase()}${s.slice(1)}${/[.!?]$/.test(s) ? '' : '.'}` : '';
