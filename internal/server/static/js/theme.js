// Follow the OS until the user chooses a palette. No server state is changed.
(function () {
  const key = 'kyrecovery-theme';
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  function stored() { try { const value = localStorage.getItem(key); return value === 'light' || value === 'dark' ? value : 'system'; } catch { return 'system'; } }
  let choice = stored();
  function apply() {
    const theme = choice === 'system' ? (media.matches ? 'dark' : 'light') : choice;
    document.documentElement.dataset.theme = theme === 'dark' ? 'busnes-dark' : 'busnes-light';
    document.querySelectorAll('[data-theme-select]').forEach(select => { select.value = choice; });
  }
  apply();
  media.addEventListener('change', () => { if (choice === 'system') apply(); });
  window.addEventListener('storage', event => { if (event.key === key || event.key === null) { choice = stored(); apply(); } });
  document.addEventListener('DOMContentLoaded', () => {
    apply();
    document.querySelectorAll('[data-theme-select]').forEach(select => select.addEventListener('change', () => {
      choice = select.value;
      try { localStorage.setItem(key, choice); } catch { /* Storage is optional. */ }
      apply();
    }));
  });
})();
