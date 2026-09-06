/**
 * The layer key rules, mirroring internal/domain/model/layerkey.go.
 *
 * Duplicated deliberately: the browser has to say "that key will be rejected"
 * while the author is still typing, and a round trip per keystroke to learn
 * that is worse than two copies of a rule that has no reason to change. The Go
 * side stays authoritative — it is what actually refuses a save.
 */

/** C#'s reserved words. Contextual keywords (var, async, record) are legal
 *  identifiers there and are deliberately absent. */
export const CSHARP_KEYWORDS = new Set([
  'abstract', 'as', 'base', 'bool', 'break', 'byte', 'case', 'catch', 'char',
  'checked', 'class', 'const', 'continue', 'decimal', 'default', 'delegate',
  'do', 'double', 'else', 'enum', 'event', 'explicit', 'extern', 'false',
  'finally', 'fixed', 'float', 'for', 'foreach', 'goto', 'if', 'implicit',
  'in', 'int', 'interface', 'internal', 'is', 'lock', 'long', 'namespace',
  'new', 'null', 'object', 'operator', 'out', 'override', 'params', 'private',
  'protected', 'public', 'readonly', 'ref', 'return', 'sbyte', 'sealed',
  'short', 'sizeof', 'stackalloc', 'static', 'string', 'struct', 'switch',
  'this', 'throw', 'true', 'try', 'typeof', 'uint', 'ulong', 'unchecked',
  'unsafe', 'ushort', 'using', 'virtual', 'void', 'volatile', 'while',
]);

/** Why the key is unusable, or '' when it is fine. */
export function validateLayerKey(key: string): string {
  if (key === '') return 'key is required';
  for (let i = 0; i < key.length; i++) {
    const c = key[i];
    if ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c === '_') continue;
    if (c >= '0' && c <= '9') {
      if (i === 0) return 'key cannot start with a digit';
      continue;
    }
    return 'key may contain only letters, digits and underscores';
  }
  if (CSHARP_KEYWORDS.has(key)) return 'key is a C# reserved word';
  return '';
}

const isAllUpper = (w: string) => /[A-Z]/.test(w) && !/[a-z]/.test(w);

/** Turns a display name into a camelCase key: 'CT Rule' -> 'ctRule'. */
export function deriveLayerKey(name: string): string {
  const words = name.match(/[A-Za-z0-9]+/g);
  if (!words || words.length === 0) return '';

  let key = words
    .map((w, i) => {
      if (i === 0) {
        // An all-caps first word reads badly kept as-is ('CT Rule' ->
        // 'CTRule'), so it is lowered whole; a mixed-case one keeps its shape.
        return isAllUpper(w) ? w.toLowerCase() : w[0].toLowerCase() + w.slice(1);
      }
      const rest = isAllUpper(w) && w.length > 1 ? w.slice(1).toLowerCase() : w.slice(1);
      return w[0].toUpperCase() + rest;
    })
    .join('');

  if (key[0] >= '0' && key[0] <= '9') key = '_' + key;
  if (CSHARP_KEYWORDS.has(key)) key += 'Layer';
  return key;
}
