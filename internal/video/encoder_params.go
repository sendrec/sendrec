package video

// ffmpeg sizes four kinds of thread pool from the visible CPU count, and each
// one is capped by a different flag in a different position. Every encode here
// runs in the process that serves HTTP, so an uncapped pool means peak memory
// tracks the node rather than the settings, and a job that outgrows the pod
// drops every in-flight request rather than just the edit. That is #200.
//
// The pools, and the helper that caps each:
//
//	input decoders    inputThreads()     immediately before every -i
//	filter graphs     globalThreads()    once, before the first -i
//	video encoder     encoderThreads()   after the inputs, before the output
//	libx264 internals x264MemoryParams() after the inputs, before the output
//
// Three review rounds landed bounds that did not bind, every one of them from
// getting a flag's position or scope wrong rather than its value, so prefer
// these helpers over hand-placed flags. Measured effect of the last round's two
// gaps, varying only the CPUs the container can see:
//
//	                        1 CPU              4 CPUs
//	composite, one -i cap   354 MB, 7 threads  357 MB, 12 threads
//	composite, per-input    353 MB, 11         353 MB, 11
//	vp9, no encoder cap     404 MB             425 MB
//	vp9, encoder capped     425 MB             425 MB
//
// The vp9 encoder was the larger leak: 21 MB of node-dependence, now 0.1 MB.
// Composite's own RSS spread was small on that fixture because the webcam input
// is small, so the decoder thread count settling at a constant is the clearer
// evidence there.
const ffmpegThreadCap = "4"

// globalThreads caps the simple and complex filter graphs. Both are global
// options, so they are emitted once at the front regardless of which graph a
// given command builds; ffmpeg accepts the unused one without complaint.
func globalThreads() []string {
	return []string{"-filter_threads", ffmpegThreadCap, "-filter_complex_threads", ffmpegThreadCap}
}

// inputThreads caps one input decoder. -threads is a per-file option that
// resets between files, so it reaches only the -i that follows it: a command
// with two inputs needs two copies. Emitting it once before the first input
// left composite's webcam decoder scaling with the node.
func inputThreads() []string {
	return []string{"-threads", ffmpegThreadCap}
}

// encoderThreads caps the video encoder, for every codec. libx264 takes roughly
// 1.5 threads per visible CPU and libvpx-vp9 takes one per CPU, so both need
// this; an earlier version applied it only alongside the libx264 parameters and
// left every vp9 output uncapped.
func encoderThreads() []string {
	return []string{"-threads", ffmpegThreadCap}
}

// x264MemoryParams bounds libx264's own allocations.
//
// rc-lookahead holds decoded frames for rate control, ref holds reference
// frames, and B-frames add a reorder buffer over both. They are work as well as
// memory, so dropping them also encodes faster — roughly 540 MB down to 340 MB,
// and about a quarter off the wall time, on a 1080p30 source.
//
// Capping threads lower saves roughly another 60 MB for about 1.5x the wall
// time, which is why the cap sits at 4 rather than 1.
//
// Figures here are approximate on purpose. They move with the ffmpeg build, the
// input and the machine, so run hack/encoder-memory/run.sh for numbers that
// describe your environment. The ratios are the finding; the megabytes are one
// sample.
//
// Output grows about 11% at the same CRF on synthetic high-entropy content, so
// real screen recordings should fare better. profile=high and level=5.1 survive
// the raw parameters, confirmed with ffprobe.
func x264MemoryParams() []string {
	return []string{"-x264-params", "rc-lookahead=10:ref=1:bframes=0"}
}

// appendEncoderBounds adds the output-side caps. Call it after the inputs and
// before the output URL: ffmpeg binds an option to the file that follows it, so
// anything after the output is parsed as trailing and discarded with a warning
// the encoder never sees.
//
// The libx264 parameters are codec-specific — libvpx-vp9 rejects the flag — but
// the thread cap applies to both.
func appendEncoderBounds(args []string, contentType string) []string {
	args = append(args, encoderThreads()...)
	if videoCodecForContentType(contentType) == "libx264" {
		args = append(args, x264MemoryParams()...)
	}
	return args
}
