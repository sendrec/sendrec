// Background service worker for SendRec Chrome Extension
// Handles recording state and upload logic

let recordingState = {
  status: 'idle', // idle, recording, paused, uploading, done, error
  startTime: null,
  pausedDuration: 0,
  pauseStart: null,
  elapsed: 0,
  progress: 0,
  shareUrl: null,
  error: null,
  mediaRecorder: null,
  recordedChunks: [],
  webcamChunks: [],
  stream: null,
  webcamStream: null
};

// We use an offscreen document to access MediaRecorder in MV3
let offscreenDocReady = false;

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  switch (msg.type) {
    case 'GET_STATE':
      sendResponse(getPublicState());
      return false;

    case 'GET_CONFIG':
      getAuthConfig().then(config => sendResponse(config));
      return true;

    case 'START_RECORDING':
      startRecording(msg.options).then(state => sendResponse(state));
      return true; // async

    case 'PAUSE_RECORDING':
      pauseRecording();
      sendResponse(getPublicState());
      return false;

    case 'STOP_RECORDING':
      stopRecording().then(state => sendResponse(state));
      return true;

    case 'OFFSCREEN_UPLOAD_PROGRESS':
      recordingState.progress = msg.progress;
      broadcastState();
      return false;

    case 'OFFSCREEN_RECORDING_STARTED':
      recordingState.status = 'recording';
      recordingState.startTime = Date.now();
      broadcastState();
      return false;

    case 'OFFSCREEN_UPLOAD_DONE':
      recordingState.progress = 100;
      recordingState.status = 'done';
      recordingState.shareUrl = msg.shareUrl;
      broadcastState();
      return false;

    case 'OFFSCREEN_UPLOAD_ERROR':
      recordingState.status = 'error';
      recordingState.error = msg.error;
      broadcastState();
      return false;
  }
});

function getPublicState() {
  return {
    status: recordingState.status,
    startTime: recordingState.startTime,
    pausedDuration: recordingState.pausedDuration,
    elapsed: recordingState.elapsed,
    progress: recordingState.progress,
    shareUrl: recordingState.shareUrl,
    error: recordingState.error
  };
}

function broadcastState() {
  const state = getPublicState();
  chrome.runtime.sendMessage({ type: 'STATE_UPDATE', state }).catch(() => {});

  // Update badge to indicate recording state
  if (state.status === 'recording' || state.status === 'paused') {
    chrome.action.setBadgeText({ text: state.status === 'paused' ? '⏸' : 'REC' });
    chrome.action.setBadgeBackgroundColor({ color: '#e53e3e' });
  } else {
    chrome.action.setBadgeText({ text: '' });
  }
}

async function startRecording(options) {
  try {
    recordingState = {
      status: 'starting',
      startTime: null,
      pausedDuration: 0,
      pauseStart: null,
      elapsed: 0,
      progress: 0,
      shareUrl: null,
      error: null,
      recordedChunks: [],
      webcamChunks: []
    };

    // Create offscreen document for recording
    await ensureOffscreenDocument();

    // Send start message to offscreen document
    await chrome.runtime.sendMessage({
      type: 'OFFSCREEN_START',
      target: 'offscreen',
      options
    });

    broadcastState();
    return getPublicState();
  } catch (err) {
    recordingState.status = 'error';
    recordingState.error = err.message;
    broadcastState();
    return getPublicState();
  }
}

function pauseRecording() {
  if (recordingState.status === 'recording') {
    recordingState.status = 'paused';
    recordingState.pauseStart = Date.now();
    recordingState.elapsed = Math.floor(
      (Date.now() - recordingState.startTime - recordingState.pausedDuration) / 1000
    );
    chrome.runtime.sendMessage({ type: 'OFFSCREEN_PAUSE', target: 'offscreen' }).catch(() => {});
  } else if (recordingState.status === 'paused') {
    recordingState.pausedDuration += Date.now() - recordingState.pauseStart;
    recordingState.pauseStart = null;
    recordingState.status = 'recording';
    chrome.runtime.sendMessage({ type: 'OFFSCREEN_RESUME', target: 'offscreen' }).catch(() => {});
  }
  broadcastState();
}

async function stopRecording() {
  if (recordingState.status === 'paused') {
    recordingState.pausedDuration += Date.now() - recordingState.pauseStart;
  }
  recordingState.elapsed = Math.floor(
    (Date.now() - recordingState.startTime - recordingState.pausedDuration) / 1000
  );
  recordingState.status = 'uploading';
  broadcastState();

  // Tell offscreen to stop — it will handle upload directly
  chrome.runtime.sendMessage({ type: 'OFFSCREEN_STOP', target: 'offscreen' }).catch(() => {});
  return getPublicState();
}

async function ensureOffscreenDocument() {
  const existingContexts = await chrome.runtime.getContexts({
    contextTypes: ['OFFSCREEN_DOCUMENT']
  });

  if (existingContexts.length > 0) return;

  await chrome.offscreen.createDocument({
    url: chrome.runtime.getURL('offscreen/offscreen.html'),
    reasons: ['USER_MEDIA', 'DISPLAY_MEDIA'],
    justification: 'Recording screen and microphone for SendRec upload'
  });
}

// --- Auth token management ---

function isTokenExpired(token) {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
    // Consider expired if less than 60s remaining
    return payload.exp * 1000 < Date.now() + 60000;
  } catch {
    return true;
  }
}

async function refreshAccessToken(serverUrl) {
  const res = await fetch(`${serverUrl}/api/auth/refresh`, {
    method: 'POST',
    credentials: 'include'
  });
  if (!res.ok) {
    throw new Error('Session expired. Please sign in again in extension settings.');
  }
  const data = await res.json();
  const newToken = data.accessToken;
  await chrome.storage.local.set({ accessToken: newToken });
  return newToken;
}

async function getAuthConfig() {
  const syncConfig = await chrome.storage.sync.get(['serverUrl']);
  const localConfig = await chrome.storage.local.get(['accessToken', 'popupWorkspace']);

  const serverUrl = syncConfig.serverUrl || 'https://app.sendrec.eu';
  let token = localConfig.accessToken;

  if (!token) {
    return { serverUrl, accessToken: null, error: 'Not signed in. Open extension settings to sign in.' };
  }

  // Auto-refresh if expired
  if (isTokenExpired(token)) {
    try {
      token = await refreshAccessToken(serverUrl);
    } catch (e) {
      return { serverUrl, accessToken: null, error: e.message };
    }
  }

  return { serverUrl, accessToken: token, organizationId: localConfig.popupWorkspace || null };
}
