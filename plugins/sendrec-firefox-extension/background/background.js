// Background script for SendRec Firefox Extension
// Recording happens in the capture tab; background handles state + upload.

let recordingState = {
  status: 'idle',
  startTime: null,
  pausedDuration: 0,
  pauseStart: null,
  elapsed: 0,
  progress: 0,
  shareUrl: null,
  error: null
};

let captureTabId = null;

browser.runtime.onMessage.addListener((msg, sender) => {
  switch (msg.type) {
    case 'GET_STATE':
      return Promise.resolve(getPublicState());

    case 'GET_CONFIG':
      return getAuthConfig();

    case 'PAUSE_RECORDING':
      pauseRecording();
      return Promise.resolve(getPublicState());

    case 'STOP_RECORDING':
      return stopRecording();
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
  browser.runtime.sendMessage({ type: 'STATE_UPDATE', state }).catch(() => {});

  // Update badge to indicate recording state
  if (state.status === 'recording' || state.status === 'paused') {
    browser.browserAction.setBadgeText({ text: state.status === 'paused' ? '⏸' : 'REC' });
    browser.browserAction.setBadgeBackgroundColor({ color: '#e53e3e' });
  } else {
    browser.browserAction.setBadgeText({ text: '' });
  }
}

// Called directly from capture page via getBackgroundPage()
// Recording is done in the capture tab; this just manages state.
function startRecordingSession(options, sourceTabId) {
  captureTabId = sourceTabId || null;

  recordingState = {
    status: 'recording',
    startTime: Date.now(),
    pausedDuration: 0,
    pauseStart: null,
    elapsed: 0,
    progress: 0,
    shareUrl: null,
    error: null
  };
  broadcastState();
}

// Called by capture tab when user clicks "Stop sharing" in browser
function onStreamEnded() {
  if (recordingState.status === 'recording' || recordingState.status === 'paused') {
    if (recordingState.status === 'paused') {
      recordingState.pausedDuration += Date.now() - recordingState.pauseStart;
    }
    recordingState.elapsed = Math.floor(
      (Date.now() - recordingState.startTime - recordingState.pausedDuration) / 1000
    );
    recordingState.status = 'uploading';
    broadcastState();
  }
}

// Called by capture tab when recording blobs are ready
async function handleRecordingBlobs(screenBlob, webcamBlob) {
  if (recordingState.status !== 'uploading') {
    // Calculate elapsed if not already done (normal stop via button)
    if (recordingState.startTime) {
      if (recordingState.status === 'paused') {
        recordingState.pausedDuration += Date.now() - recordingState.pauseStart;
      }
      recordingState.elapsed = Math.floor(
        (Date.now() - recordingState.startTime - recordingState.pausedDuration) / 1000
      );
    }
    recordingState.status = 'uploading';
    broadcastState();
  }

  if (!screenBlob || screenBlob.size === 0) {
    recordingState.status = 'error';
    recordingState.error = 'Recording was empty';
    broadcastState();
    cleanup();
    return;
  }

  const mimeType = screenBlob.type || 'video/webm';

  try {
    await uploadToSendRec(screenBlob, webcamBlob, mimeType);
  } catch (err) {
    recordingState.status = 'error';
    recordingState.error = err.message;
    broadcastState();
  }

  cleanup();
}

function pauseRecording() {
  if (recordingState.status === 'recording') {
    recordingState.status = 'paused';
    recordingState.pauseStart = Date.now();
    recordingState.elapsed = Math.floor(
      (Date.now() - recordingState.startTime - recordingState.pausedDuration) / 1000
    );
    // Tell capture tab to pause recorders
    browser.runtime.sendMessage({ type: 'CAPTURE_PAUSE' }).catch(() => {});
  } else if (recordingState.status === 'paused') {
    recordingState.pausedDuration += Date.now() - recordingState.pauseStart;
    recordingState.pauseStart = null;
    recordingState.status = 'recording';
    browser.runtime.sendMessage({ type: 'CAPTURE_RESUME' }).catch(() => {});
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

  // Tell capture tab to stop recorders — their onstop handlers will call handleRecordingBlobs
  browser.runtime.sendMessage({ type: 'CAPTURE_STOP' }).catch(() => {});

  return getPublicState();
}

function cleanup() {
  // Close the capture tab — this stops all streams since they belong to that tab
  if (captureTabId) {
    browser.tabs.remove(captureTabId).catch(() => {});
    captureTabId = null;
  }
}

async function uploadToSendRec(screenBlob, webcamBlob, mimeType) {
  const config = await getAuthConfig();
  if (!config || !config.serverUrl || !config.accessToken) {
    throw new Error(config?.error || 'Not signed in. Open extension settings to sign in.');
  }

  const serverUrl = config.serverUrl.replace(/\/$/, '');
  const token = config.accessToken;
  const duration = recordingState.elapsed || 1;

  const body = {
    title: `Recording ${new Date().toLocaleString()}`,
    duration: duration,
    fileSize: screenBlob.size,
    contentType: mimeType.split(';')[0]
  };

  if (webcamBlob) {
    body.webcamFileSize = webcamBlob.size;
    body.webcamContentType = mimeType.split(';')[0];
  }

  // Step 1: Create video record
  recordingState.progress = 10;
  broadcastState();

  const createHeaders = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  };
  const orgData = await browser.storage.local.get(['popupWorkspace']);
  if (orgData.popupWorkspace) {
    createHeaders['X-Organization-Id'] = orgData.popupWorkspace;
  }

  const createRes = await fetch(`${serverUrl}/api/videos`, {
    method: 'POST',
    credentials: 'include',
    headers: createHeaders,
    body: JSON.stringify(body)
  });

  if (!createRes.ok) {
    const errText = await createRes.text();
    throw new Error(`Failed to create video: ${createRes.status} ${errText}`);
  }

  const videoData = await createRes.json();
  const { id, uploadUrl, shareToken } = videoData;

  // Step 2: Upload screen recording
  recordingState.progress = 30;
  broadcastState();

  const uploadRes = await fetch(uploadUrl, {
    method: 'PUT',
    headers: { 'Content-Type': body.contentType },
    body: screenBlob
  });

  if (!uploadRes.ok) {
    throw new Error(`Failed to upload video: ${uploadRes.status}`);
  }

  recordingState.progress = 70;
  broadcastState();

  // Upload webcam if present
  if (webcamBlob && videoData.webcamUploadUrl) {
    const wcRes = await fetch(videoData.webcamUploadUrl, {
      method: 'PUT',
      headers: { 'Content-Type': body.webcamContentType },
      body: webcamBlob
    });
    if (!wcRes.ok) {
      console.warn('Webcam upload failed:', wcRes.status);
    }
  }

  recordingState.progress = 90;
  broadcastState();

  // Step 3: Mark as ready
  await fetch(`${serverUrl}/api/videos/${id}`, {
    method: 'PATCH',
    credentials: 'include',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({ status: 'ready' })
  });

  // Done
  recordingState.progress = 100;
  recordingState.status = 'done';
  recordingState.shareUrl = `${serverUrl}/watch/${shareToken}`;
  broadcastState();
}

// --- Auth token management ---

function isTokenExpired(token) {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]));
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
  await browser.storage.local.set({ accessToken: newToken });
  return newToken;
}

async function getAuthConfig() {
  const syncConfig = await browser.storage.sync.get(['serverUrl']);
  const localConfig = await browser.storage.local.get(['accessToken']);

  const serverUrl = syncConfig.serverUrl || 'https://app.sendrec.eu';
  let token = localConfig.accessToken;

  if (!token) {
    return { serverUrl, accessToken: null, error: 'Not signed in. Open extension settings to sign in.' };
  }

  if (isTokenExpired(token)) {
    try {
      token = await refreshAccessToken(serverUrl);
    } catch (e) {
      return { serverUrl, accessToken: null, error: e.message };
    }
  }

  return { serverUrl, accessToken: token };
}

function getSupportedMimeType() {
  const types = [
    'video/webm;codecs=vp9,opus',
    'video/webm;codecs=vp8,opus',
    'video/webm;codecs=vp9',
    'video/webm;codecs=vp8',
    'video/webm'
  ];
  for (const type of types) {
    if (MediaRecorder.isTypeSupported(type)) {
      return type;
    }
  }
  return 'video/webm';
}
