document.addEventListener('DOMContentLoaded', async () => {
  // Initialize theme
  await initTheme();
  const btnTheme = document.getElementById('btn-theme');
  btnTheme.addEventListener('click', () => toggleTheme());

  const btnExit = document.getElementById('btn-exit');
  btnExit.addEventListener('click', () => window.close());

  const form = document.getElementById('settings-form');
  const serverUrl = document.getElementById('server-url');
  const email = document.getElementById('email');
  const password = document.getElementById('password');
  const btnLogout = document.getElementById('btn-logout');
  const message = document.getElementById('message');
  const loginStatus = document.getElementById('login-status');
  const loginStatusText = document.getElementById('login-status-text');

  // Load saved settings
  const config = await chrome.storage.sync.get([
    'serverUrl', 'email'
  ]);
  const localConfig = await chrome.storage.local.get(['accessToken']);

  if (config.serverUrl) serverUrl.value = config.serverUrl;
  if (config.email) email.value = config.email;

  // Show logged-in status
  if (localConfig.accessToken) {
    loginStatus.classList.remove('hidden');
    loginStatusText.textContent = `✓ Signed in as ${config.email || 'user'}`;
    loginStatusText.style.color = 'var(--color-accent)';
    btnLogout.classList.remove('hidden');
    password.placeholder = 'Leave empty to keep current session';
    password.required = false;
  }

  // Save / Sign in
  form.addEventListener('submit', async (e) => {
    e.preventDefault();

    const url = serverUrl.value.trim().replace(/\/$/, '');
    const emailVal = email.value.trim();
    const passwordVal = password.value;

    if (!url) {
      showMessage('Please enter a server URL', 'error');
      return;
    }

    if (!emailVal) {
      showMessage('Please enter your email', 'error');
      return;
    }

    // If password is provided, log in to get tokens
    if (passwordVal) {
      try {
        const res = await fetch(`${url}/api/auth/login`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          credentials: 'include',
          body: JSON.stringify({ email: emailVal, password: passwordVal })
        });

        if (!res.ok) {
          if (res.status === 401) {
            showMessage('Invalid email or password.', 'error');
          } else {
            const errText = await res.text();
            showMessage(`Login failed: ${res.status} ${errText}`, 'error');
          }
          return;
        }

        const data = await res.json();
        await chrome.storage.local.set({
          accessToken: data.accessToken
        });

        loginStatus.classList.remove('hidden');
        loginStatusText.textContent = `✓ Signed in as ${emailVal}`;
        loginStatusText.style.color = 'var(--color-accent)';
        btnLogout.classList.remove('hidden');
        password.value = '';
        password.placeholder = 'Leave empty to keep current session';
        password.required = false;
      } catch (err) {
        showMessage(`Connection failed: ${err.message}`, 'error');
        return;
      }
    } else if (!localConfig.accessToken) {
      showMessage('Please enter your password to sign in', 'error');
      return;
    }

    // Save settings
    await chrome.storage.sync.set({
      serverUrl: url,
      email: emailVal
    });

    showMessage('Settings saved!', 'success');
  });

  // Sign out
  btnLogout.addEventListener('click', async () => {
    await chrome.storage.local.remove(['accessToken']);
    loginStatus.classList.add('hidden');
    btnLogout.classList.add('hidden');
    password.placeholder = 'Your account password';
    password.required = true;
    showMessage('Signed out.', 'success');
  });

  // Media permissions
  const btnMediaPerms = document.getElementById('btn-media-permissions');

  btnMediaPerms.addEventListener('click', async () => {
    let micGranted = false;
    let camGranted = false;

    try {
      const micStream = await navigator.mediaDevices.getUserMedia({ audio: true });
      micStream.getTracks().forEach(t => t.stop());
      micGranted = true;
    } catch (e) {
      console.warn('Mic permission failed:', e);
    }

    try {
      const camStream = await navigator.mediaDevices.getUserMedia({ video: true });
      camStream.getTracks().forEach(t => t.stop());
      camGranted = true;
    } catch (e) {
      console.warn('Webcam permission failed:', e);
    }

    if (micGranted && camGranted) {
      showMessage('Media access granted!', 'success');
    } else if (micGranted) {
      showMessage('Microphone granted, but webcam was denied or not found.', 'error');
    } else if (camGranted) {
      showMessage('Webcam granted, but microphone was denied or not found.', 'error');
    } else {
      showMessage('Permission denied. Please allow access in browser settings.', 'error');
    }
  });

  function showMessage(text, type) {
    message.textContent = text;
    message.className = `message ${type}`;
    message.classList.remove('hidden');
    setTimeout(() => { message.classList.add('hidden'); }, 5000);
  }
});
