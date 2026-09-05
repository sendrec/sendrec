#!/usr/bin/env bash
# Peak RSS and wall time for the libx264 configurations in internal/video.
set -euo pipefail
cd "$(dirname "$0")"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

cat > "$WORK/Dockerfile" <<'DOCKER'
FROM alpine:3.20
COPY --from=mwader/static-ffmpeg:7.1 /ffmpeg /ffprobe /usr/local/bin/
RUN apk add --no-cache bash
WORKDIR /work
DOCKER

cat > "$WORK/peak.sh" <<'PEAK'
#!/bin/bash
label="$1"; shift
t0=$(date +%s.%N); "$@" >/dev/null 2>/tmp/err & pid=$!
peak=0
while kill -0 $pid 2>/dev/null; do
  hwm=$(awk '/VmHWM/{print $2}' /proc/$pid/status 2>/dev/null)
  [ -n "$hwm" ] && [ "$hwm" -gt "$peak" ] && peak=$hwm
  sleep 0.05
done
wait $pid; rc=$?; t1=$(date +%s.%N)
printf '%-34s rc=%-3s peak=%7.1f MB  wall=%6.1fs  ' \
  "$label" "$rc" "$(echo "$peak/1024" | bc -l)" "$(echo "$t1-$t0" | bc -l)"
grep -oE 'Trailing option|ref=[0-9]+|bframes=[0-9]+|rc_lookahead=[0-9]+' /tmp/err | tr '\n' ' '; echo
PEAK

cat > "$WORK/bench.sh" <<'BENCH'
#!/bin/bash
set -u
VF="scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2"
BOUNDS="rc-lookahead=10:ref=1:bframes=0"

echo "visible cpus: $(nproc)"
ffmpeg -nostdin -hide_banner -loglevel error \
  -f lavfi -i "testsrc2=size=1920x1080:rate=30:duration=30" \
  -f lavfi -i "sine=duration=30" \
  -c:v libx264 -preset veryfast -g 60 -c:a aac -y /work/fx.mp4

echo; echo "== encoder settings (buildTranscodeArgs shape) =="
/work/peak.sh "defaults" ffmpeg -nostdin -hide_banner -loglevel info -i /work/fx.mp4 \
  -c:v libx264 -profile:v high -level:v 5.1 -preset fast -crf 23 -vf "$VF" -r 60 \
  -c:a aac -movflags +faststart -y /work/a.mp4
/work/peak.sh "bounded" ffmpeg -nostdin -hide_banner -loglevel info -i /work/fx.mp4 \
  -c:v libx264 -profile:v high -level:v 5.1 -preset fast -crf 23 -vf "$VF" -r 60 \
  -c:a aac -movflags +faststart -x264-params "$BOUNDS" -y /work/b.mp4

echo; echo "== option ordering: ffmpeg discards options after the output =="
/work/peak.sh "bounds AFTER the output" ffmpeg -nostdin -hide_banner -loglevel info -i /work/fx.mp4 \
  -c:v libx264 -preset fast -crf 23 -an -y /work/c.mp4 -x264-params "$BOUNDS"
/work/peak.sh "bounds BEFORE the output" ffmpeg -nostdin -hide_banner -loglevel info -i /work/fx.mp4 \
  -c:v libx264 -preset fast -crf 23 -an -x264-params "$BOUNDS" -y /work/d.mp4

echo; echo "== thread count: x264 scales threads with visible CPUs =="
for t in auto 1 2 4 8; do
  if [ "$t" = auto ]; then targ=(); else targ=(-threads "$t"); fi
  /work/peak.sh "threads $t" ffmpeg -nostdin -hide_banner -loglevel info -i /work/fx.mp4 \
    -c:v libx264 -preset fast -crf 23 "${targ[@]}" -x264-params "$BOUNDS" -an -y "/work/t_$t.mp4"
done

echo; echo "== output size cost =="
ls -l /work/a.mp4 /work/b.mp4 | awk '{printf "%12d  %s\n", $5, $9}'

echo; echo "== profile and level survive the raw params =="
ffprobe -v error -select_streams v -show_entries stream=profile,level,refs,has_b_frames \
  -of default=nw=1 /work/b.mp4
BENCH

chmod +x "$WORK/peak.sh" "$WORK/bench.sh"
docker build -q -t sendrec-encoder-bench "$WORK" >/dev/null
docker run --rm -v "$WORK:/work" sendrec-encoder-bench /work/bench.sh
