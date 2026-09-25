// Resolve the actual Busnes CSS blocks and recovery aliases in both modes.
import { readFileSync } from 'node:fs';
import assert from 'node:assert/strict';
const read = path => readFileSync(new URL(path, import.meta.url), 'utf8');
const shared = read('../internal/server/static/ky-ui/tokens.css');
const css = read('../internal/server/static/css/styles.css');
const script = read('../internal/server/static/js/theme.js');
for (const page of ['index.html', 'ceremony.html']) {
  const html = read(`../internal/server/static/${page}`);
  assert(html.indexOf('/static/ky-ui/tokens.css') < html.indexOf('/static/css/styles.css'), `${page}: product semantic aliases must follow shared aliases`);
}
assert(css.includes(':root:not([data-theme]):not([data-ky-theme])'), 'System alias specificity must match shared defaults');
const declarations = text => Object.fromEntries([...text.matchAll(/--([a-z-]+):\s*([^;]+);/g)].map(m => [m[1], m[2].trim()]));
const aliases = css.match(/:root\[data-theme="busnes-dark"\]\s*\{([^}]+)\}/)[1];
assert(script.includes('busnes-dark') && script.includes('busnes-light'), 'Check selectors against the live theme script');
const luminance = hex => {
  assert.match(hex, /^#[a-f0-9]{6}$/i);
  const rgb = hex.slice(1).match(/../g).map(x => parseInt(x, 16) / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4);
  return rgb[0] * .2126 + rgb[1] * .7152 + rgb[2] * .0722;
};
for (const mode of ['light', 'dark']) {
  const block = shared.match(new RegExp(`:root\\[data-ky-theme="busnes-${mode}"\\] \\{([^}]+)`))[1];
  const values = { ...declarations(block), ...declarations(aliases) };
  const resolve = name => {
    const value = values[name];
    assert(value, `Missing ${name}`);
    const alias = value.match(/^var\(--([a-z-]+)\)$/);
    return alias ? resolve(alias[1]) : value;
  };
  for (const foreground of ['paper', 'paper-muted', 'paper-dim', 'brass', 'ok', 'warn', 'bad']) {
    const surfaces = ['ink', 'ink-raised', 'ink-well'];
    if (['brass', 'ok', 'warn', 'bad'].includes(foreground)) surfaces.push(`${foreground}-wash`);
    for (const background of surfaces) {
      const [a,b] = [luminance(resolve(foreground)), luminance(resolve(background))].sort((a,b) => b-a);
      const ratio = (a+.05)/(b+.05);
      assert(ratio >= 4.5, `${mode}: ${foreground} on ${background}: ${ratio.toFixed(2)} < 4.5`);
    }
  }
  console.log(`${mode}: active recovery palette meets AA`);
}
