package video

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Inspect encoded output, not the presence or position of an ffmpeg option.
func TestRemoveSegmentsBoundsActualOutput(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			if os.Getenv("CI") != "" {
				t.Fatal(err)
			}
			t.Skip(tool + " not installed")
		}
	}
	cut := []segmentRange{{Start: 0.5, End: 1}}
	denseCuts := make([]segmentRange, 200)
	for i := range denseCuts {
		denseCuts[i] = segmentRange{Start: float64(1+i*9) / 1000, End: float64(4+i*9) / 1000}
	}
	packetCuts := make([]segmentRange, 40)
	for i := range packetCuts {
		at := float64(2+i*2) * 1024 / 48000
		packetCuts[i] = segmentRange{Start: at - 0.001, End: at + 0.001}
	}
	frameCuts := make([]segmentRange, 120)
	for i := range frameCuts {
		at := float64(i) / 60
		frameCuts[i] = segmentRange{Start: math.Max(0, at-0.002), End: at + 0.002}
	}
	for _, tc := range []struct {
		name, size, contentType  string
		fps, maxWidth, maxHeight int
		audio                    bool
		segments                 []segmentRange
		wantFrames               int
	}{
		{"high fps mp4", "320x240", "video/mp4", 1000, 320, 240, true, cut, 90},
		{"high fps quicktime", "320x240", "video/quicktime", 1000, 320, 240, true, cut, 90},
		{"high fps webm", "320x240", "video/webm", 1000, 320, 240, true, cut, 90},
		{"high DPI", "2304x1296", "video/mp4", 5, 1920, 1080, false, cut, 90},
		{"ordinary recording", "320x240", "video/mp4", 30, 320, 240, false, cut, 90},
		{"dense cuts", "320x240", "video/mp4", 1000, 320, 240, true, denseCuts, 84},
		{"audio packet boundaries", "320x240", "video/mp4", 60, 320, 240, true, packetCuts, 115},
		{"retained frames between output frames", "320x240", "video/mp4", 1000, 320, 240, false, frameCuts, 91},
		{"reject empty video", "320x240", "video/mp4", 60, 320, 240, false, frameCuts, 0},
		{"cut from start", "320x240", "video/mp4", 1000, 320, 240, true, []segmentRange{{Start: 0, End: 0.5}}, 90},
		{"cut at end", "320x240", "video/mp4", 1000, 320, 240, true, []segmentRange{{Start: 1.5, End: 2}}, 90},
		{"adjacent cuts", "320x240", "video/mp4", 1000, 320, 240, true, []segmentRange{{Start: 0.5, End: 0.75}, {Start: 0.75, End: 1}}, 90},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			dir := t.TempDir()
			input := filepath.Join(dir, "input.mp4")
			args := []string{"-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("color=size=%s:rate=%d:duration=2", tc.size, tc.fps)}
			if tc.audio {
				args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=2", "-c:a", "aac")
			}
			args = append(args, "-c:v", "libx264", "-threads", "1", "-preset", "ultrafast", "-pix_fmt", "yuv420p", input)
			if out, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
				t.Fatalf("fixture: %v: %s", err, out)
			}
			output := filepath.Join(dir, "output"+extensionForContentType(tc.contentType))
			err := removeSegmentsFromVideo(ctx, input, output, tc.contentType, tc.segments, tc.audio)
			if tc.wantFrames == 0 {
				if err == nil {
					t.Fatal("an empty edit must fail before it can replace the original")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-count_frames", "-show_entries", "stream=codec_type,width,height,r_frame_rate,nb_read_frames,duration:format=duration", "-of", "json", output).Output()
			if err != nil {
				t.Fatal(err)
			}
			var probe struct {
				Streams []struct {
					CodecType     string `json:"codec_type"`
					Width, Height int
					Rate          string `json:"r_frame_rate"`
					Frames        string `json:"nb_read_frames"`
					Duration      string
				}
				Format struct{ Duration string }
			}
			if err := json.Unmarshal(out, &probe); err != nil {
				t.Fatal(err)
			}
			duration, err := strconv.ParseFloat(probe.Format.Duration, 64)
			wantDuration := float64(tc.wantFrames) / 60
			if err != nil || math.Abs(duration-wantDuration) > 0.05 {
				t.Errorf("retained duration = %q, want about %g seconds", probe.Format.Duration, wantDuration)
			}
			var videoSeen, audioSeen bool
			for _, stream := range probe.Streams {
				if stream.CodecType == "audio" {
					audioSeen = true
					// MP4/MOV expose per-stream duration; format.duration alone
					// hides an audio stream shortened by packet-sized cuts.
					if stream.Duration != "" {
						audioDuration, err := strconv.ParseFloat(stream.Duration, 64)
						if err != nil || math.Abs(audioDuration-wantDuration) > 0.025 {
							t.Errorf("audio duration = %q, want about %g seconds", stream.Duration, wantDuration)
						}
					}
				}
				if stream.CodecType != "video" {
					continue
				}
				videoSeen = true
				if stream.Width != tc.maxWidth || stream.Height != tc.maxHeight {
					t.Errorf("output dimensions = %dx%d, want %dx%d", stream.Width, stream.Height, tc.maxWidth, tc.maxHeight)
				}
				var numerator, denominator float64
				if _, err := fmt.Sscanf(stream.Rate, "%f/%f", &numerator, &denominator); err != nil || denominator == 0 {
					t.Fatalf("invalid frame rate %q", stream.Rate)
				}
				rate := numerator / denominator
				if rate != 60 {
					t.Errorf("encoded frame rate = %g, want 60", rate)
				}
				frames, err := strconv.Atoi(stream.Frames)
				// Allow one frame of timestamp quantization for the whole output,
				// not one frame per cut (the dense fixture caught that drift).
				if err != nil || math.Abs(float64(frames-tc.wantFrames)) > 1 {
					t.Errorf("decoded %s frames, want %d ± 1 (cut rounding must not accumulate)", stream.Frames, tc.wantFrames)
				}
			}
			if !videoSeen || audioSeen != tc.audio {
				t.Errorf("output streams: video=%t audio=%t, want video=true audio=%t", videoSeen, audioSeen, tc.audio)
			}
		})
	}
}
