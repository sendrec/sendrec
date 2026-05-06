(async () => {
  const status = document.getElementById('status');
  const params = new URLSearchParams(window.location.search);
  const needsMic = params.get('mic') === '1';
  const needsWebcam = params.get('webcam') === '1';

  const constraints = {
    audio: needsMic,
    video: needsWebcam ? { width: 320, height: 240, facingMode: 'user' } : false
  };

  try {
    const stream = await navigator.mediaDevices.getUserMedia(constraints);
    stream.getTracks().forEach(t => t.stop());
    status.textContent = '✓ Permissions granted! You can close this tab.';
    status.className = 'status success';

    // Notify background that permissions were granted
    chrome.runtime.sendMessage({ type: 'PERMISSIONS_GRANTED' });

    // Auto-close after a short delay
    setTimeout(() => window.close(), 1500);
  } catch (err) {
    status.textContent = '✗ Permission denied. Please click the camera icon in the address bar to allow access.';
    status.className = 'status error';
  }
})();
