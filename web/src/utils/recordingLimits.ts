export const MIN_RECORDING_SECONDS = 1;
export const MIN_RECORDING_BYTES = 1024;

// A capture stalled longer than this cost the recording real content: the video
// sits on one frame while the audio runs on. Shorter blips are not worth
// throwing a recording away for.
// ponytail: flat threshold, not a share of the recording — revisit if short
// clips start being rejected over a stall they could absorb.
export const MAX_CAPTURE_STALL_MS = 2000;
