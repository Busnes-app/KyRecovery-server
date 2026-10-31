// Follow the OS until the user chooses a palette. No server state is changed.
// The suite catalog comes from ky-ui's classic themes-browser.js, loaded first in <head>,
// so the saved choice applies before first paint. The saved value is "system" (Busnes
// following the OS), a pair ID (that pair following the OS) or a half ID (a fixed mode).
// Busnes halves keep their original saved values ("light", "dark"); removed IDs fall back
// to "system".
(() => {
'use strict';
const { SYSTEM_THEME, THEME_PAIRS, THEME_PALETTES, findPair, findTheme } = window.kyThemes;

const key = 'kyrecovery-theme';
const legacy = { light: 'busnes-light', dark: 'busnes-dark' };
const saved = { 'busnes-light': 'light', 'busnes-dark': 'dark' };
const media = window.matchMedia('(prefers-color-scheme: dark)');
// KyCalendar's reference: per pair, Match system / Light / Dark. Busnes's Match system is "system".
const choices = THEME_PAIRS.map(pair => ({
  pair,
  modes: [
    { label: 'Match system', value: pair.id === 'busnes-pair' ? SYSTEM_THEME.id : pair.id },
    { label: 'Light', value: pair.light },
    { label: 'Dark', value: pair.dark },
  ],
}));

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
  try {
    const value = localStorage.getItem(key);
    const id = legacy[value] ?? value;
    return findPair(id)?.id ?? findTheme(id)?.id ?? SYSTEM_THEME.id;
  } catch { return SYSTEM_THEME.id; }
}
let choice = stored();

function apply() {
  const root = document.documentElement;
  const id = choice === SYSTEM_THEME.id ? (media.matches ? 'busnes-dark' : 'busnes-light') : findTheme(choice, media.matches).id;
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

// One native radio group across the pair cards: arrow keys move through every choice.
function picker(grid) {
  for (const { pair, modes } of choices) {
    const card = document.createElement('div');
    card.className = 'appearance-pair';
    card.setAttribute('role', 'group');
    card.setAttribute('aria-labelledby', `${pair.id}-name`);
    const head = document.createElement('div');
    head.className = 'appearance-pair-head';
    const name = document.createElement('h2');
    name.id = `${pair.id}-name`;
    name.className = 'appearance-pair-name';
    name.textContent = pair.name;
    const halves = document.createElement('span');
    halves.className = 'appearance-pair-halves';
    halves.textContent = `${findTheme(pair.light).name} · ${findTheme(pair.dark).name}`;
    head.append(name, halves);
    const preview = document.createElement('span');
    preview.className = 'ky-appearance-preview';
    preview.setAttribute('aria-hidden', 'true');
    preview.dataset.kyPreview = pair.id;
    const row = document.createElement('div');
    row.className = 'appearance-modes';
    for (const mode of modes) {
      const option = document.createElement('label');
      option.className = 'appearance-mode';
      const radio = document.createElement('input');
      radio.type = 'radio';
      radio.name = key;
      radio.value = mode.value;
      radio.addEventListener('change', () => choose(mode.value));
      // KyCalendar's ✓ before the selected label; CSS shows it only on the checked mode.
      const check = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      check.setAttribute('viewBox', '0 0 24 24');
      check.setAttribute('aria-hidden', 'true');
      check.classList.add('appearance-check');
      const tick = document.createElementNS('http://www.w3.org/2000/svg', 'path');
      tick.setAttribute('d', 'M20 6 9 17l-5-5');
      check.append(tick);
      const label = document.createElement('span');
      label.textContent = mode.label;
      option.append(radio, check, label);
      row.append(option);
    }
    card.append(head, preview, row);
    grid.append(card);
  }
}

// The sign-in switcher lists the same choices, grouped by pair.
function shortcut(select) {
  select.replaceChildren(...choices.map(({ pair, modes }) => {
    const group = document.createElement('optgroup');
    group.label = pair.name;
    group.append(...modes.map(mode => new Option(`${pair.name} · ${mode.label}`, mode.value)));
    return group;
  }));
  select.addEventListener('change', () => choose(select.value));
}

media.addEventListener('change', () => { if (choice === SYSTEM_THEME.id || findPair(choice)) apply(); });
window.addEventListener('storage', event => { if (event.key === key || event.key === null) { choice = stored(); apply(); } });
apply();
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('[data-theme-picker] .ky-appearance-grid').forEach(picker);
  document.querySelectorAll('[data-theme-select]').forEach(shortcut);
  apply();
});
})();
