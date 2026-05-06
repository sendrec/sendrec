// Offscreen document handles actual MediaRecorder APIs
// since service workers can't access getUserMedia/getDisplayMedia

let screenRecorder = null;
let webcamRecorder = null;
let screenChunks = [];
let webcamChunks = [];
let screenStream = null;
let webcamStream = null;

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.target !== 'offscreen') return;

  switch (msg.type) {
    case 'OFFSCREEN_START':
      handleStart(msg.options);
      break;
    case 'OFFSCREEN_PAUSE':
      handlePause();
      break;
    case 'OFFSCREEN_RESUME':
      handleResume();
      break;
    case 'OFFSCREEN_STOP':
      handleStop();
      break;
  }
});

async function handleStart(options) {
  try {
    screenChunks = [];
    webcamChunks = [];

    const includesScreen = options.source === 'screen' || options.source === 'tab';
    const includesWebcam = options.webcam;

    // Get screen/tab stream
    if (includesScreen) {
      const displayMediaOptions = {
        video: true,
        audio: false
      };

      screenStream = await navigator.mediaDevices.getDisplayMedia(displayMediaOptions);

      // Add microphone if requested
      if (options.micAudio) {
        try {
          const micStream = await navigator.mediaDevices.getUserMedia({
            audio: { echoCancellation: true, noiseSuppression: true },
            video: false
          });

          // Create combined stream: screen video + mic audio
          const combinedStream = new MediaStream([
            ...screenStream.getVideoTracks(),
            ...micStream.getAudioTracks()
          ]);

          screenStream = combinedStream;
        } catch (micErr) {
          console.warn('Microphone access denied, recording without mic:', micErr);
        }
      }

      screenRecorder = new MediaRecorder(screenStream, {
        mimeType: getSupportedMimeType(),
        videoBitsPerSecond: 2500000
      });

      screenRecorder.ondataavailable = (e) => {
        if (e.data.size > 0) {
          screenChunks.push(e.data);
        }
      };

      screenRecorder.onstop = () => {
        finishRecording();
      };

      // Stop if user clicks "Stop sharing" in browser UI
      screenStream.getVideoTracks()[0].onended = () => {
        handleStop();
      };

      screenRecorder.start(1000); // Collect data every second

      // Notify background that recording has actually started
      chrome.runtime.sendMessage({ type: 'OFFSCREEN_RECORDING_STARTED' });
    }

    // Get webcam stream
    if (includesWebcam) {
      try {
        webcamStream = await navigator.mediaDevices.getUserMedia({
          video: { width: 320, height: 240, facingMode: 'user' },
          audio: !includesScreen && options.micAudio // Only capture audio on webcam if no screen
        });

        webcamRecorder = new MediaRecorder(webcamStream, {
          mimeType: getSupportedMimeType(),
          videoBitsPerSecond: 800000
        });

        webcamRecorder.ondataavailable = (e) => {
          if (e.data.size > 0) {
            webcamChunks.push(e.data);
          }
        };

        webcamRecorder.start(1000);
      } catch (webcamErr) {
        console.warn('Webcam access denied:', webcamErr);
      }
    }

    // Webcam-only mode
    if (!includesScreen && includesWebcam) {
      if (!webcamRecorder) {
        throw new Error('Webcam access denied');
      }
      // Use webcam as the main recorder for stop handling
      screenRecorder = webcamRecorder;
      screenChunks = webcamChunks;
      webcamRecorder = null;
      webcamChunks = [];

      screenRecorder.onstop = () => {
        finishRecording();
      };

      // Notify background that recording has actually started (webcam-only)
      chrome.runtime.sendMessage({ type: 'OFFSCREEN_RECORDING_STARTED' });
    }
  } catch (err) {
    const msg = (err.name === 'NotAllowedError' || (err.message && err.message.includes('Permission denied')))
      ? 'Screen sharing was cancelled.'
      : (err.message || 'Recording cancelled');
    chrome.runtime.sendMessage({
      type: 'OFFSCREEN_UPLOAD_ERROR',
      error: msg
    });
  }
}

function handlePause() {
  if (screenRecorder && screenRecorder.state === 'recording') {
    screenRecorder.pause();
  }
  if (webcamRecorder && webcamRecorder.state === 'recording') {
    webcamRecorder.pause();
  }
}

function handleResume() {
  if (screenRecorder && screenRecorder.state === 'paused') {
    screenRecorder.resume();
  }
  if (webcamRecorder && webcamRecorder.state === 'paused') {
    webcamRecorder.resume();
  }
}

function handleStop() {
  if (screenRecorder && screenRecorder.state !== 'inactive') {
    screenRecorder.stop();
  }
  if (webcamRecorder && webcamRecorder.state !== 'inactive') {
    webcamRecorder.stop();
  }

  // Stop all tracks
  if (screenStream) {
    screenStream.getTracks().forEach(t => t.stop());
  }
  if (webcamStream) {
    webcamStream.getTracks().forEach(t => t.stop());
  }
}

async function finishRecording() {
  // Small delay to ensure all chunks are collected
  await new Promise(r => setTimeout(r, 200));

  const mimeType = getSupportedMimeType();
  const screenBlob = new Blob(screenChunks, { type: mimeType });

  // Don't upload if recording was empty (cancelled immediately)
  if (screenBlob.size === 0) {
    chrome.runtime.sendMessage({
      type: 'OFFSCREEN_UPLOAD_ERROR',
      error: 'Recording was empty'
    });
    cleanup();
    return;
  }

  let webcamBlob = null;
  if (webcamChunks.length > 0) {
    webcamBlob = new Blob(webcamChunks, { type: mimeType });
  }

  // Upload directly from offscreen (avoids message size limits)
  try {
    await uploadToSendRec(screenBlob, webcamBlob, mimeType);
  } catch (err) {
    chrome.runtime.sendMessage({
      type: 'OFFSCREEN_UPLOAD_ERROR',
      error: err.message
    });
  }

  cleanup();
}

function cleanup() {
  screenRecorder = null;
  webcamRecorder = null;
  screenChunks = [];
  webcamChunks = [];
  screenStream = null;
  webcamStream = null;
}

async function uploadToSendRec(screenBlob, webcamBlob, mimeType) {
  const config = await chrome.runtime.sendMessage({ type: 'GET_CONFIG' });
  if (!config || !config.serverUrl || !config.accessToken) {
    throw new Error(config?.error || 'Not signed in. Open extension settings to sign in.');
  }

  const serverUrl = config.serverUrl.replace(/\/$/, '');
  const token = config.accessToken;

  // Get duration from background
  const stateRes = await chrome.runtime.sendMessage({ type: 'GET_STATE' });
  const duration = stateRes.elapsed || 1;

  const body = {
    title: `Recording ${new Date().toLocaleString()}`,
    duration: duration,
    fileSize: screenBlob.size,
    contentType: mimeType.split(';')[0] // Use base mime type without codecs
  };

  if (webcamBlob) {
    body.webcamFileSize = webcamBlob.size;
    body.webcamContentType = mimeType.split(';')[0];
  }

  // Step 1: Create video record
  chrome.runtime.sendMessage({ type: 'OFFSCREEN_UPLOAD_PROGRESS', progress: 10 });

  const createHeaders = {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  };
  if (config.organizationId) {
    createHeaders['X-Organization-Id'] = config.organizationId;
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

  // Step 2: Upload screen recording to presigned URL
  chrome.runtime.sendMessage({ type: 'OFFSCREEN_UPLOAD_PROGRESS', progress: 30 });

  const uploadRes = await fetch(uploadUrl, {
    method: 'PUT',
    headers: { 'Content-Type': body.contentType },
    body: screenBlob
  });

  if (!uploadRes.ok) {
    throw new Error(`Failed to upload video: ${uploadRes.status}`);
  }

  chrome.runtime.sendMessage({ type: 'OFFSCREEN_UPLOAD_PROGRESS', progress: 70 });

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

  chrome.runtime.sendMessage({ type: 'OFFSCREEN_UPLOAD_PROGRESS', progress: 90 });

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
  chrome.runtime.sendMessage({
    type: 'OFFSCREEN_UPLOAD_DONE',
    shareUrl: `${serverUrl}/watch/${shareToken}`
  });
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
