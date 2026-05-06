// capture.js — Opens in a tab, acquires all streams, records locally, keeps tab alive during recording
(async () => {
  const params = new URLSearchParams(window.location.search);
  const options = JSON.parse(params.get('options') || '{}');
  const statusEl = document.getElementById('status');
  const btnCapture = document.getElementById('btn-capture');

  btnCapture.addEventListener('click', async () => {
    btnCapture.disabled = true;
    btnCapture.textContent = 'Waiting for selection...';
    statusEl.textContent = '';

    try {
      const includesScreen = options.source === 'screen' || options.source === 'tab';
      let capturedScreenStream = null;
      let capturedMicStream = null;
      let capturedWebcamStream = null;

      if (includesScreen) {
        capturedScreenStream = await navigator.mediaDevices.getDisplayMedia({
          video: true,
          audio: false
        });
      }

      // Acquire mic
      if (options.micAudio && includesScreen) {
        try {
          capturedMicStream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
        } catch (e) {
          console.warn('Mic access denied:', e);
        }
      }

      // Acquire webcam
      if (options.webcam) {
        try {
          capturedWebcamStream = await navigator.mediaDevices.getUserMedia({
            video: { width: 320, height: 240, facingMode: 'user' },
            audio: !includesScreen && options.micAudio
          });
        } catch (e) {
          console.warn('Webcam access denied:', e);
        }
      }

      // Build the final recording stream — attach mic audio to screen video if available
      let recordingStream = null;
      if (includesScreen && capturedScreenStream) {
        if (capturedMicStream) {
          recordingStream = new MediaStream([
            ...capturedScreenStream.getVideoTracks(),
            ...capturedMicStream.getAudioTracks()
          ]);
        } else {
          recordingStream = capturedScreenStream;
        }
      }

      const bgPage = await browser.runtime.getBackgroundPage();
      const thisTab = await browser.tabs.getCurrent();

      // Pick mimeType based on whether audio is actually present in the recording stream.
      // Using an opus codec with a video-only stream causes Firefox's MediaRecorder to fail silently.
      const hasAudio = recordingStream ? recordingStream.getAudioTracks().length > 0 : false;
      const mimeType = hasAudio
        ? (MediaRecorder.isTypeSupported('video/webm;codecs=vp8,opus') ? 'video/webm;codecs=vp8,opus' : 'video/webm')
        : (MediaRecorder.isTypeSupported('video/webm;codecs=vp8') ? 'video/webm;codecs=vp8' : 'video/webm');

      // --- Record screen locally in capture tab ---
      let screenRecorder = null;
      const screenChunks = [];
      let screenBlob = null;

      // --- Record webcam locally ---
      let webcamRecorder = null;
      const webcamChunks = [];
      let webcamBlob = null;

      // Tracks completion of both recorders before uploading
      let screenDone = !recordingStream; // true if no screen to record
      let webcamDone = !capturedWebcamStream; // true if no webcam to record

      function tryUpload() {
        if (screenDone && webcamDone) {
          const mainBlob = screenBlob || webcamBlob;
          const secondaryBlob = screenBlob ? webcamBlob : null;
          bgPage.handleRecordingBlobs(mainBlob, secondaryBlob);
        }
      }

      if (recordingStream) {
        screenRecorder = new MediaRecorder(recordingStream, {
          mimeType,
          videoBitsPerSecond: 2500000
        });

        screenRecorder.ondataavailable = (e) => {
          if (e.data.size > 0) {
            screenChunks.push(e.data);
          }
        };

        screenRecorder.onstop = () => {
          screenBlob = new Blob(screenChunks, { type: mimeType });
          screenDone = true;
          tryUpload();
        };

        // If user clicks "Stop sharing" in browser UI
        recordingStream.getVideoTracks()[0].onended = () => {
          if (screenRecorder.state !== 'inactive') {
            screenRecorder.stop();
          }
          if (webcamRecorder && webcamRecorder.state !== 'inactive') {
            webcamRecorder.stop();
          }
          bgPage.onStreamEnded();
        };

        screenRecorder.start(1000);
      }

      if (capturedWebcamStream) {
        webcamRecorder = new MediaRecorder(capturedWebcamStream, {
          mimeType,
          videoBitsPerSecond: 800000
        });

        webcamRecorder.ondataavailable = (e) => {
          if (e.data.size > 0) {
            webcamChunks.push(e.data);
          }
        };

        webcamRecorder.onstop = () => {
          webcamBlob = new Blob(webcamChunks, { type: mimeType });
          webcamDone = true;
          tryUpload();
        };

        webcamRecorder.start(1000);
      }

      // Tell background we started (for state management)
      const webcamOnly = !includesScreen && options.webcam;
      bgPage.startRecordingSession(options, thisTab.id);

      // Listen for control messages from background (stop/pause)
      browser.runtime.onMessage.addListener((msg) => {
        if (msg.type === 'CAPTURE_STOP') {
          if (screenRecorder && screenRecorder.state !== 'inactive') {
            screenRecorder.stop();
          }
          if (webcamRecorder && webcamRecorder.state !== 'inactive') {
            webcamRecorder.stop();
          }
          // If no recorders were active, trigger upload directly
          if ((!screenRecorder || screenRecorder.state === 'inactive') &&
              (!webcamRecorder || webcamRecorder.state === 'inactive')) {
            tryUpload();
          }
        } else if (msg.type === 'CAPTURE_PAUSE') {
          if (screenRecorder && screenRecorder.state === 'recording') screenRecorder.pause();
          if (webcamRecorder && webcamRecorder.state === 'recording') webcamRecorder.pause();
        } else if (msg.type === 'CAPTURE_RESUME') {
          if (screenRecorder && screenRecorder.state === 'paused') screenRecorder.resume();
          if (webcamRecorder && webcamRecorder.state === 'paused') webcamRecorder.resume();
        }
      });

      // Show minimal recording state — this tab must stay open to keep streams alive
      document.querySelector('.container').innerHTML = `
        <h2 style="color:#00b67a;">● Recording</h2>
        <p style="color:#94a3b8;font-size:13px;">This tab keeps the capture alive.<br>It will close automatically when recording stops.</p>
      `;

      // Switch focus back to the previous tab
      const tabs = await browser.tabs.query({ currentWindow: true });
      const prevTab = tabs.find(t => t.index === thisTab.index - 1);
      if (prevTab) {
        browser.tabs.update(prevTab.id, { active: true });
      }
    } catch (err) {
      if (err.name === 'NotAllowedError') {
        statusEl.textContent = 'Screen sharing was cancelled.';
      } else {
        statusEl.textContent = err.message || 'Failed to start recording';
      }
      btnCapture.disabled = false;
      btnCapture.textContent = 'Start Screen Recording';
    }
  });
})();
