package video

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	silenceStartRegex = regexp.MustCompile(`silence_start:\s*([\d.]+)`)
	silenceEndRegex   = regexp.MustCompile(`silence_end:\s*([\d.]+)`)
)

func parseSilenceDetectOutput(stderr string) []segmentRange {
	var segments []segmentRange
	var pendingStart float64
	hasPendingStart := false

	for _, line := range strings.Split(stderr, "\n") {
		if match := silenceStartRegex.FindStringSubmatch(line); match != nil {
			start, err := strconv.ParseFloat(match[1], 64)
			if err != nil {
				continue
			}
			pendingStart = start
			hasPendingStart = true
			continue
		}

		if match := silenceEndRegex.FindStringSubmatch(line); match != nil && hasPendingStart {
			end, err := strconv.ParseFloat(match[1], 64)
			if err != nil {
				continue
			}
			segments = append(segments, segmentRange{Start: pendingStart, End: end})
			hasPendingStart = false
		}
	}

	return segments
}

func detectSilence(inputPath string, noiseDB int, minDuration float64) ([]segmentRange, error) {
	filterValue := fmt.Sprintf("silencedetect=noise=%ddB:d=%.2f", noiseDB, minDuration)
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-af", filterValue,
		"-f", "null",
		"-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg silencedetect: %w: %s", err, string(output))
	}

	return parseSilenceDetectOutput(string(output)), nil
}
