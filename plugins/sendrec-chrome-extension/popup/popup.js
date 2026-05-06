document.addEventListener('DOMContentLoaded', async () => {
  // Initialize theme
  await initTheme();
  const btnTheme = document.getElementById('btn-theme');
  btnTheme.addEventListener('click', () => toggleTheme());

  const btnStart = document.getElementById('btn-start');
  const btnPause = document.getElementById('btn-pause');
  const btnStop = document.getElementById('btn-stop');
  const statusIndicator = document.getElementById('status-indicator');
  const statusText = document.getElementById('status-text');
  const timer = document.getElementById('timer');
  const uploadStatus = document.getElementById('upload-status');
  const progressFill = document.getElementById('progress-fill');
  const uploadText = document.getElementById('upload-text');
  const resultDiv = document.getElementById('result');
  const shareLink = document.getElementById('share-link');
  const btnCopyLink = document.getElementById('btn-copy-link');
  const notConfigured = document.getElementById('not-configured');
  const openOptions = document.getElementById('open-options');
  const btnSettings = document.getElementById('btn-settings');
  const webcam = document.getElementById('webcam');
  const micAudio = document.getElementById('mic-audio');
  const workspaceSelect = document.getElementById('workspace-select');

  // Check configuration
  const config = await chrome.storage.sync.get(['serverUrl', 'email']);
  if (!config.serverUrl || !config.email) {
    notConfigured.classList.remove('hidden');
  }

  // Restore saved popup selections
  const saved = await chrome.storage.local.get(['popupWebcam', 'popupMic', 'popupWorkspace']);
  let webcamOn = saved.popupWebcam !== undefined ? saved.popupWebcam : true;
  let micOn = saved.popupMic !== undefined ? saved.popupMic : true;

  function setToggle(btn, on) {
    btn.classList.toggle('btn-toggle--active', on);
    if (btn.id === 'webcam') btn.textContent = on ? 'Camera On' : 'Camera Off';
    if (btn.id === 'mic-audio') btn.textContent = on ? 'Microphone On' : 'Microphone Off';
  }
  setToggle(webcam, webcamOn);
  setToggle(micAudio, micOn);

  // Save selections on change
  function saveSelections() {
    chrome.storage.local.set({
      popupWebcam: webcamOn,
      popupMic: micOn,
      popupWorkspace: workspaceSelect.value
    });
  }
  webcam.addEventListener('click', () => {
    webcamOn = !webcamOn;
    setToggle(webcam, webcamOn);
    saveSelections();
  });
  micAudio.addEventListener('click', () => {
    micOn = !micOn;
    setToggle(micAudio, micOn);
    saveSelections();
  });
  workspaceSelect.addEventListener('change', saveSelections);

  // Load workspaces
  async function loadWorkspaces() {
    const cfg = await chrome.runtime.sendMessage({ type: 'GET_CONFIG' });
    if (!cfg || !cfg.accessToken || !cfg.serverUrl) return;
    try {
      const res = await fetch(`${cfg.serverUrl.replace(/\/$/, '')}/api/organizations`, {
        headers: { 'Authorization': `Bearer ${cfg.accessToken}` }
      });
      if (res.ok) {
        const orgs = await res.json();
        orgs.forEach(org => {
          const opt = document.createElement('option');
          opt.value = org.id;
          opt.textContent = org.name;
          workspaceSelect.appendChild(opt);
        });
        if (saved.popupWorkspace) workspaceSelect.value = saved.popupWorkspace;
      }
    } catch (e) {
      console.warn('Failed to load workspaces:', e);
    }
  }
  loadWorkspaces();

  openOptions.addEventListener('click', (e) => {
    e.preventDefault();
    chrome.runtime.openOptionsPage();
  });

  btnSettings.addEventListener('click', () => {
    chrome.runtime.openOptionsPage();
  });

  document.getElementById('logo-link').addEventListener('click', async () => {
    const cfg = await chrome.storage.sync.get(['serverUrl']);
    const url = cfg.serverUrl || 'https://app.sendrec.eu';
    chrome.tabs.create({ url });
  });

  // Restore state from background
  let timerInterval = null;
  const state = await chrome.runtime.sendMessage({ type: 'GET_STATE' });
  updateUI(state);

  btnStart.addEventListener('click', async () => {
    const options = {
      source: 'screen',
      webcam: webcamOn,
      micAudio: micOn,
      organizationId: workspaceSelect.value || null
    };

    // Check if we need mic/camera permissions and if they're already granted
    const needsMic = options.micAudio;
    const needsWebcam = options.webcam;
    if (needsMic || needsWebcam) {
      const permsToCheck = [];
      if (needsMic) permsToCheck.push(navigator.permissions.query({ name: 'microphone' }));
      if (needsWebcam) permsToCheck.push(navigator.permissions.query({ name: 'camera' }));

      const results = await Promise.all(permsToCheck);
      const needsPrompt = results.some(r => r.state !== 'granted');

      if (needsPrompt) {
        // Open permissions page in a new tab (popups can't show permission dialogs)
        const params = `mic=${needsMic ? 1 : 0}&webcam=${needsWebcam ? 1 : 0}`;
        chrome.tabs.create({
          url: chrome.runtime.getURL(`permissions/permissions.html?${params}`)
        });
        statusText.textContent = 'Grant permissions in the new tab, then try again';
        return;
      }
    }

    const response = await chrome.runtime.sendMessage({ type: 'START_RECORDING', options });
    if (response.error) {
      statusText.textContent = response.error;
      return;
    }
    updateUI(response);
  });

  btnPause.addEventListener('click', async () => {
    const response = await chrome.runtime.sendMessage({ type: 'PAUSE_RECORDING' });
    updateUI(response);
  });

  btnStop.addEventListener('click', async () => {
    const response = await chrome.runtime.sendMessage({ type: 'STOP_RECORDING' });
    updateUI(response);
  });

  btnCopyLink.addEventListener('click', () => {
    const link = shareLink.href;
    navigator.clipboard.writeText(link).then(() => {
      btnCopyLink.textContent = 'Copied!';
      setTimeout(() => { btnCopyLink.textContent = 'Copy Link'; }, 2000);
    });
  });

  // Listen for state updates from background
  chrome.runtime.onMessage.addListener((msg) => {
    if (msg.type === 'STATE_UPDATE') {
      updateUI(msg.state);
    }
  });

  function updateUI(state) {
    if (!state) return;

    clearInterval(timerInterval);

    switch (state.status) {
      case 'idle':
        statusIndicator.className = 'indicator idle';
        statusText.textContent = 'Ready';
        timer.textContent = '00:00';
        btnStart.classList.remove('hidden');
        btnPause.classList.add('hidden');
        btnStop.classList.add('hidden');
        btnStart.disabled = false;
        uploadStatus.classList.add('hidden');
        resultDiv.classList.add('hidden');
        break;

      case 'starting':
        statusIndicator.className = 'indicator idle';
        statusText.textContent = 'Select a window...';
        timer.textContent = '00:00';
        btnStart.classList.remove('hidden');
        btnPause.classList.add('hidden');
        btnStop.classList.add('hidden');
        btnStart.disabled = true;
        uploadStatus.classList.add('hidden');
        resultDiv.classList.add('hidden');
        break;

      case 'recording':
        statusIndicator.className = 'indicator recording';
        statusText.textContent = 'Recording';
        btnStart.classList.add('hidden');
        btnPause.classList.remove('hidden');
        btnStop.classList.remove('hidden');
        btnPause.disabled = false;
        btnStop.disabled = false;
        btnPause.innerHTML = '<svg width="16" height="16" viewBox="0 0 16 16"><rect x="3" y="3" width="4" height="10" fill="currentColor"/><rect x="9" y="3" width="4" height="10" fill="currentColor"/></svg> Pause';
        startTimer(state.startTime, state.pausedDuration || 0);
        break;

      case 'paused':
        statusIndicator.className = 'indicator paused';
        statusText.textContent = 'Paused';
        btnStart.classList.add('hidden');
        btnPause.classList.remove('hidden');
        btnStop.classList.remove('hidden');
        btnPause.disabled = false;
        btnStop.disabled = false;
        btnPause.innerHTML = '<svg width="16" height="16" viewBox="0 0 16 16"><polygon points="4,2 14,8 4,14" fill="currentColor"/></svg> Resume';
        if (state.elapsed) {
          timer.textContent = formatTime(state.elapsed);
        }
        break;

      case 'uploading':
        statusIndicator.className = 'indicator uploading';
        statusText.textContent = 'Uploading...';
        btnStart.classList.add('hidden');
        btnPause.classList.add('hidden');
        btnStop.classList.add('hidden');
        uploadStatus.classList.remove('hidden');
        resultDiv.classList.add('hidden');
        if (state.progress !== undefined) {
          progressFill.style.width = state.progress + '%';
          uploadText.textContent = `Uploading... ${state.progress}%`;
        }
        break;

      case 'done':
        statusIndicator.className = 'indicator idle';
        statusText.textContent = 'Done';
        btnStart.classList.remove('hidden');
        btnPause.classList.add('hidden');
        btnStop.classList.add('hidden');
        btnStart.disabled = false;
        uploadStatus.classList.add('hidden');
        resultDiv.classList.remove('hidden');
        if (state.shareUrl) {
          shareLink.href = state.shareUrl;
          shareLink.textContent = state.shareUrl;
        }
        break;

      case 'error':
        statusIndicator.className = 'indicator idle';
        statusText.textContent = state.error || 'Error';
        btnStart.classList.remove('hidden');
        btnPause.classList.add('hidden');
        btnStop.classList.add('hidden');
        btnStart.disabled = false;
        uploadStatus.classList.add('hidden');
        break;
    }
  }

  function startTimer(startTime, pausedDuration) {
    updateTimerDisplay(startTime, pausedDuration);
    timerInterval = setInterval(() => {
      updateTimerDisplay(startTime, pausedDuration);
    }, 1000);
  }

  function updateTimerDisplay(startTime, pausedDuration) {
    const elapsed = Math.floor((Date.now() - startTime - pausedDuration) / 1000);
    timer.textContent = formatTime(elapsed);
  }

  function formatTime(seconds) {
    const m = Math.floor(seconds / 60).toString().padStart(2, '0');
    const s = (seconds % 60).toString().padStart(2, '0');
    return `${m}:${s}`;
  }
});
