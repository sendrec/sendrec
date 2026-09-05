# Encoder memory harness

Reproduces the measurements behind `x264MemoryParams` in `internal/video/encoder_params.go`.

Needs Docker. Nothing is installed on the host; ffmpeg runs from a scratch image.

```sh
./run.sh
```

Roughly 10 minutes on 4 cores, plus a few more for the CPU-visibility sweep. It builds a 1080p30 fixture, then reports peak RSS
and wall time per variant, sampling `VmHWM` from `/proc/<pid>/status` every 50ms.

Covers the encoder settings, option ordering, thread count, the argument shape
the app actually passes, both composite inputs, the vp9 output encoder, stream
copy as a floor, and a sweep that varies how many CPUs the container can see.
That last one is the only section that can show whether a bound is a bound: a
figure from one machine cannot.

Peak RSS is the ffmpeg process only. It is the number that matters for these
settings, but it is not the pod's total footprint — the Go server sits alongside
it, and nothing in the app bounds how many encodes run at once.

The fixture is `testsrc2`, which is high-motion synthetic content. It is a
conservative stand-in for screen recordings, which are mostly static: expect
lower absolute numbers on real input, and treat the ratios as the finding rather
than the absolute megabytes.
