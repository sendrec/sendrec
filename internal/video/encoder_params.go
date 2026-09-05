package video

// x264MemoryParams bounds libx264's own allocations.
//
// Every encode in this package runs in the process that serves HTTP, so peak
// resident memory is an availability concern: a job that needs more than the
// pod has takes live traffic down with it, not just the edit.
//
// rc-lookahead holds decoded frames for rate control, ref holds reference
// frames, and B-frames add a reorder buffer over both. They are work as well as
// memory, so dropping them also encodes faster — roughly 540 MB down to 340 MB,
// and a third off the wall time, on a 1080p30 source.
//
// Figures here are approximate on purpose. They move with the ffmpeg build, the
// input and the machine, so run hack/encoder-memory/run.sh for numbers that
// describe your environment rather than trusting these. The ratios are the
// finding; the megabytes are one sample.
//
// Output grows about 11% at the same CRF on synthetic high-entropy content, so
// real screen recordings should fare better. profile=high and level=5.1 survive
// the raw parameters, confirmed with ffprobe.
//
// Thread count is capped at 4 because x264 otherwise takes roughly 1.5 threads
// per visible CPU and each thread holds frame buffers. Uncapped, the ceiling
// tracks the node instead of the settings. Four costs nothing here — within a
// second of uncapped on 4 CPUs — while going lower saves about 60 MB for nearly
// double the wall time.
func x264MemoryParams() []string {
	return []string{"-threads", "4", "-x264-params", "rc-lookahead=10:ref=1:bframes=0"}
}

// appendX264Params adds the bounds only when the content type resolves to
// libx264; libvpx-vp9 rejects -x264-params outright.
//
// Call this before the output URL is appended. ffmpeg applies options to the
// output that follows them, so options placed after the last one are parsed as
// trailing and discarded with a warning — the encoder then runs on defaults,
// with nothing failing to show it.
func appendX264Params(args []string, contentType string) []string {
	if videoCodecForContentType(contentType) != "libx264" {
		return args
	}
	return append(args, x264MemoryParams()...)
}

// ffmpegPipelineThreads bounds the thread pools ffmpeg sizes from the visible
// CPU count, and must precede the first -i.
//
// Capping the encoder alone leaves two other pools unbounded: the input decoder
// and the simple and complex filter graphs each default to the available CPU
// count independently. Same command, same x264 header, varying only how many
// CPUs the container can see:
//
//	                encoder cap only   whole pipeline
//	1 CPU              268 MB             296 MB
//	2 CPUs             287 MB             296 MB
//	4 CPUs             300 MB             296 MB
//
// The encoder-only column tracks the node. The whole-pipeline column does not,
// which is the entire point: a bound that holds on the machine it was measured
// on is not a bound. Measured to 4 CPUs, which is what the test machine had, so
// the flatness is demonstrated rather than proven for larger nodes.
//
// The trade is visible in the first row — forcing 4 threads on a 1-CPU pod costs
// about 28 MB over letting it size itself. A fixed ceiling everywhere is worth
// more than the smallest possible footprint on the smallest possible node.
//
// Placement is what makes these work. ffmpeg binds an option to the next file,
// so -threads must precede -i to reach the input decoder; after the input it
// configures the encoder instead. -filter_threads and -filter_complex_threads
// are global and set here for the same reason.
//
// Unlike the x264 bounds these apply to every encode, vp9 included: decoding and
// filtering happen whatever the output codec is.
func ffmpegPipelineThreads() []string {
	return []string{"-threads", "4", "-filter_threads", "4", "-filter_complex_threads", "4"}
}
