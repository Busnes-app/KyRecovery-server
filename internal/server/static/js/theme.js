// Follow the OS until the user chooses a palette. No server state is changed.
// The suite catalog comes from ky-ui's classic themes-browser.js, loaded first in <head>,
// so the saved choice applies before first paint. Busnes keeps its original saved values
// ("system", "light", "dark"); other themes save their catalog ID.
(() => {
'use strict';
const { SYSTEM_THEME, THEME_CATALOG, THEME_PALETTES, findTheme } = window.kyThemes;

const key = 'kyrecovery-theme';
const legacy = { light: 'busnes-light', dark: 'busnes-dark' };
const saved = { 'busnes-light': 'light', 'busnes-dark': 'dark' };
const options = [SYSTEM_THEME, ...THEME_CATALOG];
const media = window.matchMedia('(prefers-color-scheme: dark)');

// KyRecovery's local --ink is its surface colour, so the shared helper's product
// variables would collide. Its stylesheet reads the --ky-* tokens instead; a named
// palette overrides those inline, under the matching Busnes scheme.
const tokens = {
  bg: ['--ky-bg'], panel: ['--ky-panel'], panelHover: ['--ky-panel-hover'], ink: ['--ky-ink', '--ky-ink-muted'],
  inkStrong: ['--ky-ink-strong'], accent: ['--ky-accent'], accentSoft: ['--ky-accent-soft'], line: ['--ky-line'],
  lineStrong: ['--ky-line-strong'], buttonText: ['--ky-button-text'], sidebarStart: ['--ky-sidebar-start'],
  sidebarEnd: ['--ky-sidebar-end'], danger: ['--ky-danger'], success: ['--ky-success'],
};

function stored() {
  try { const value = localStorage.getItem(key); return findTheme(legacy[value] ?? value)?.id ?? 'system'; } catch { return 'system'; }
}
let choice = stored();

function apply() {
  const root = document.documentElement;
  const id = choice === 'system' ? (media.matches ? 'busnes-dark' : 'busnes-light') : choice;
  const palette = THEME_PALETTES[id];
  for (const names of Object.values(tokens)) for (const name of names) root.style.removeProperty(name);
  if (palette) for (const [field, names] of Object.entries(tokens)) for (const name of names) if (palette[field]) root.style.setProperty(name, palette[field]);
  const scheme = findTheme(id).scheme;
  root.dataset.theme = `busnes-${scheme}`;
  root.style.colorScheme = scheme;
  document.querySelectorAll('[data-theme-select]').forEach(select => { select.value = choice; });
  document.querySelectorAll('[data-theme-picker] input').forEach(radio => { radio.checked = radio.value === choice; });
}

function choose(id) {
  choice = id;
  try { localStorage.setItem(key, saved[id] ?? id); } catch { /* Storage is optional. */ }
  apply();
}

// Native radio cards built with DOM calls; ky-ui's stylesheets draw them and their previews.
function picker(grid) {
  for (const theme of options) {
    const option = document.createElement('label');
    option.className = 'ky-appearance-option';
    const radio = document.createElement('input');
    radio.className = 'ky-appearance-radio';
    radio.type = 'radio';
    radio.name = 'kyrecovery-theme';
    radio.value = theme.id;
    radio.addEventListener('change', () => choose(theme.id));
    const preview = document.createElement('span');
    preview.className = 'ky-appearance-preview';
    preview.setAttribute('aria-hidden', 'true');
    preview.dataset.kyPreview = theme.id;
    const name = document.createElement('span');
    name.className = 'ky-appearance-name';
    name.textContent = theme.name;
    const mode = document.createElement('small');
    mode.className = 'ky-appearance-mode';
    mode.textContent = theme.scheme === 'system' ? 'Follows device' : theme.scheme === 'light' ? 'Light' : 'Dark';
    option.append(radio, preview, name, mode);
    grid.append(option);
  }
}

// The header shortcut lists the same catalog.
function shortcut(select) {
  select.replaceChildren(...options.map(theme => new Option(theme.name, theme.id)));
  select.addEventListener('change', () => choose(select.value));
}

media.addEventListener('change', () => { if (choice === 'system') apply(); });
window.addEventListener('storage', event => { if (event.key === key || event.key === null) { choice = stored(); apply(); } });
apply();
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('[data-theme-picker] .ky-appearance-grid').forEach(picker);
  document.querySelectorAll('[data-theme-select]').forEach(shortcut);
  apply();
});
})();
