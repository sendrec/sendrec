package video

// x264MemoryParams bounds libx264's peak resident memory.
//
// Every encode in this package runs in the process that serves HTTP, so peak
// RSS is an availability concern rather than a tuning detail: a 1080p job that
// needs more than the pod has takes live traffic down with it, not just the
// edit.
//
// Two independent allocation drivers. Every figure below comes from
// hack/encoder-memory/run.sh — a 30s 1080p30 source, ffmpeg 7.1, 4 visible CPUs
// — so they can be re-run rather than taken on trust.
//
// Encoder settings. rc-lookahead holds decoded frames for rate control, ref
// holds reference frames, and B-frames add a reorder buffer over both. They are
// work as well as memory, so dropping them also encodes faster:
//
//	defaults   524 MB   34s
//	bounded    342 MB   23s
//
// Thread count. Left alone x264 picks roughly 1.5 threads per visible CPU, and
// each thread holds frame buffers, so an uncapped encoder allocates more on a
// bigger node — the bound would hold on a 4-core box and quietly fail on a
// 32-core one:
//
//	threads auto   347 MB   16s
//	threads 1      242 MB   30s
//	threads 2      264 MB   21s
//	threads 4      306 MB   17s
//	threads 8      389 MB   17s
//
// Four is the cap because it decouples the ceiling from the node without paying
// for it: on this box it is within a second of uncapped, and on a larger one it
// holds where uncapped would keep climbing. Going lower buys another 60 MB for
// nearly double the wall time.
//
// Output grows 11% at the same CRF on synthetic high-entropy content, so real
// screen recordings should fare better. profile=high and level=5.1 come through
// unchanged, confirmed with ffprobe.
func x264MemoryParams() []string {
	return []string{"-threads", "4", "-x264-params", "rc-lookahead=10:ref=1:bframes=0"}
}

// appendX264Params adds the bounds only when the content type resolves to
// libx264; libvpx-vp9 rejects -x264-params outright.
//
// Call this before the output URL is appended. ffmpeg applies options to the
// output that follows them, so options placed after the last one are parsed as
// trailing and discarded with a warning — the encoder then runs on defaults,
// 530 MB instead of 347 MB, with nothing failing to show it.
func appendX264Params(args []string, contentType string) []string {
	if videoCodecForContentType(contentType) != "libx264" {
		return args
	}
	return append(args, x264MemoryParams()...)
}
