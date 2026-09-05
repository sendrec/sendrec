package video

// x264MemoryParams bounds libx264's peak resident memory.
//
// Every encode in this package runs in the process that serves HTTP, so peak
// RSS is an availability concern rather than a tuning detail: a 1080p job that
// needs more than the pod has takes live traffic down with it, not just the
// edit. Measured on a 1080p30 source with ffmpeg 7.1, running the argument list
// buildTranscodeArgs produces:
//
//	defaults      523 MB   283s
//	these params  338 MB   135s
//
// Isolating the settings on a plain encode of the same source put the defaults
// at 622 MB and these at 350 MB, with -threads 2 and -threads 1 reaching 267 MB
// and 244 MB.
//
// The three settings are the allocation drivers: rc-lookahead holds decoded
// frames for rate control, ref holds reference frames, and B-frames add a
// reorder buffer on top of both. They are work as well as memory, which is why
// dropping them also halves the wall time. The cost is 11% larger output at the
// same CRF on synthetic high-entropy content, so real screen recordings should
// fare better. profile=high and level=5.1 come through unchanged.
//
// Thread count is deliberately left at the ffmpeg default. Capping it saves
// another 106 MB but doubles wall time, and every caller here runs under a
// 10 minute context — trading an OOM for a timeout is not a fix.
func x264MemoryParams() []string {
	return []string{"-x264-params", "rc-lookahead=10:ref=1:bframes=0"}
}

// appendX264Params adds the bounds only when the content type resolves to
// libx264; libvpx-vp9 rejects -x264-params outright.
func appendX264Params(args []string, contentType string) []string {
	if videoCodecForContentType(contentType) != "libx264" {
		return args
	}
	return append(args, x264MemoryParams()...)
}
