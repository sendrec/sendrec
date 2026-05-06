// Shared theme utility for SendRec Firefox Extension
// Syncs theme preference across popup and options via browser.storage

async function initTheme() {
  const { theme } = await browser.storage.sync.get('theme');
  const resolved = theme || 'dark';
  applyTheme(resolved);
  return resolved;
}

function applyTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
  const moonIcon = document.getElementById('icon-moon');
  const sunIcon = document.getElementById('icon-sun');
  if (moonIcon && sunIcon) {
    if (theme === 'dark') {
      moonIcon.classList.remove('hidden');
      sunIcon.classList.add('hidden');
    } else {
      moonIcon.classList.add('hidden');
      sunIcon.classList.remove('hidden');
    }
  }
}

async function toggleTheme() {
  const { theme } = await browser.storage.sync.get('theme');
  const current = theme || 'dark';
  const next = current === 'dark' ? 'light' : 'dark';
  await browser.storage.sync.set({ theme: next });
  applyTheme(next);
  return next;
}
