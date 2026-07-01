package video

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	vttVoiceRe         = regexp.MustCompile(`^<v\s+([^>]+)>`)
	vttSpanTagRe       = regexp.MustCompile(`</?[a-zA-Z][^>]*>|<\d{2,}:[0-5]\d:[0-5]\d\.\d{3}>`)
	leadingTimestampRe = regexp.MustCompile(`^\d{1,2}:\d{2}`)
)

// parseVTT parses a WebVTT file from Teams, Zoom, or a generic source into
// TranscriptSegments, tolerating BOM, CRLF/CR line endings, optional cue
// identifiers, HTML entities, and voice/span tags.
func parseVTT(raw []byte) ([]TranscriptSegment, error) {
	text := string(raw)
	const bom = "\xef\xbb\xbf"
	text = strings.TrimPrefix(text, bom)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	trimmed := strings.TrimLeft(text, " \t")
	if !strings.HasPrefix(trimmed, "WEBVTT") {
		return nil, fmt.Errorf("not a WebVTT file: missing WEBVTT signature")
	}
	after := trimmed[len("WEBVTT"):]
	if after != "" && !strings.HasPrefix(after, "\n") && !strings.HasPrefix(after, " ") && !strings.HasPrefix(after, "\t") {
		return nil, fmt.Errorf("not a WebVTT file: malformed signature")
	}

	blocks := strings.Split(text, "\n\n")
	var segments []TranscriptSegment
	for _, block := range blocks {
		lines := strings.Split(strings.Trim(block, "\n"), "\n")
		if len(lines) == 0 {
			continue
		}
		first := strings.TrimSpace(lines[0])
		if strings.HasPrefix(first, "WEBVTT") || strings.HasPrefix(first, "NOTE") ||
			strings.HasPrefix(first, "STYLE") || strings.HasPrefix(first, "REGION") {
			continue
		}
		// A leading non-timing line is an opaque cue identifier (Teams GUID,
		// Zoom integer). Discard it.
		idx := 0
		if _, _, ok := parseTimingLine(lines[idx]); !ok {
			idx++
		}
		if idx >= len(lines) {
			continue
		}
		start, end, ok := parseTimingLine(lines[idx])
		if !ok {
			continue
		}
		payload := strings.Join(lines[idx+1:], "\n")
		if strings.TrimSpace(payload) == "" {
			continue
		}
		speaker, cleaned := extractSpeaker(payload)
		speaker = sanitizeSpeaker(speaker)
		// Unescape entities before stripping tags so entity-encoded markup
		// (e.g. &lt;v X&gt;) is stripped rather than resurrected into live VTT
		// markup in the re-rendered output.
		cleaned = html.UnescapeString(cleaned)
		cleaned = vttSpanTagRe.ReplaceAllString(cleaned, "")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			continue
		}
		segments = append(segments, TranscriptSegment{
			Start: start, End: end, Text: cleaned, Speaker: speaker,
		})
	}

	if len(segments) == 0 {
		return nil, fmt.Errorf("no cues found in WebVTT file")
	}
	return segments, nil
}

func parseTimingLine(line string) (start, end float64, ok bool) {
	parts := strings.SplitN(line, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	// The end field may carry cue settings after the timestamp; take field 0.
	endField := strings.Fields(strings.TrimSpace(parts[1]))
	if len(endField) == 0 {
		return 0, 0, false
	}
	s, ok1 := parseVTTTimestamp(strings.TrimSpace(parts[0]))
	e, ok2 := parseVTTTimestamp(endField[0])
	if !ok1 || !ok2 {
		return 0, 0, false
	}
	return s, e, true
}

func parseVTTTimestamp(s string) (float64, bool) {
	dot := strings.LastIndex(s, ".")
	if dot < 0 || len(s)-dot != 4 {
		return 0, false
	}
	ms, err := strconv.Atoi(s[dot+1:])
	if err != nil {
		return 0, false
	}
	hms := strings.Split(s[:dot], ":")
	var h, m, sec int
	var errH, errM, errS error
	switch len(hms) {
	case 3:
		h, errH = strconv.Atoi(hms[0])
		m, errM = strconv.Atoi(hms[1])
		sec, errS = strconv.Atoi(hms[2])
	case 2:
		m, errM = strconv.Atoi(hms[0])
		sec, errS = strconv.Atoi(hms[1])
	default:
		return 0, false
	}
	// Reject non-numeric components (silent Atoi zeroing would keep a cue at
	// the wrong time), but tolerate out-of-range numeric values that real
	// exporters occasionally emit rather than dropping the cue.
	if errH != nil || errM != nil || errS != nil || h < 0 || m < 0 || sec < 0 {
		return 0, false
	}
	return float64(h)*3600 + float64(m)*60 + float64(sec) + float64(ms)/1000, true
}

// speakerSanitizer strips characters that would corrupt a re-emitted <v
// SPEAKER> voice tag; a real display name never contains them.
var speakerSanitizer = strings.NewReplacer("<", "", ">", "", "\n", "", "\r", "")

// sanitizeSpeaker unescapes HTML entities in a speaker name (mirroring cue
// text) and strips angle brackets/newlines so the name can't break out of
// the <v SPEAKER> voice tag it's re-emitted into by segmentsToVTT.
func sanitizeSpeaker(speaker string) string {
	return speakerSanitizer.Replace(html.UnescapeString(speaker))
}

// extractSpeaker pulls a speaker name from a cue payload via a <v Name> voice
// tag or a leading "Name: " prefix (Zoom), splitting on the first ": " only.
func extractSpeaker(text string) (speaker, cleaned string) {
	if m := vttVoiceRe.FindStringSubmatch(text); m != nil {
		clean := text[len(m[0]):]
		// Strip trailing </v> if present
		clean = strings.TrimSuffix(clean, "</v>")
		return strings.TrimSpace(m[1]), clean
	}
	// Only treat a leading "Name: " on the first line as a speaker.
	nl := strings.IndexByte(text, '\n')
	firstLine := text
	rest := ""
	if nl >= 0 {
		firstLine = text[:nl]
		rest = text[nl:]
	}
	if i := strings.Index(firstLine, ": "); i > 0 {
		name := strings.TrimSpace(firstLine[:i])
		if name != "" && !strings.ContainsAny(name, "<>") && !leadingTimestampRe.MatchString(name) {
			return name, strings.TrimSpace(firstLine[i+2:]) + rest
		}
	}
	return "", text
}

// mergeSegments joins consecutive cues from the same speaker when the gap
// between them is at most maxMergeGapSeconds, extending the end time and
// concatenating text with a single space. Text pieces are accumulated per
// merged group and joined once to keep this linear in total text size.
func mergeSegments(in []TranscriptSegment) []TranscriptSegment {
	const maxMergeGapSeconds = 2.0
	var out []TranscriptSegment
	var parts [][]string
	for _, s := range in {
		if n := len(out); n > 0 && out[n-1].Speaker == s.Speaker && s.Start-out[n-1].End <= maxMergeGapSeconds {
			if s.End > out[n-1].End {
				out[n-1].End = s.End
			}
			parts[n-1] = append(parts[n-1], s.Text)
			continue
		}
		out = append(out, s)
		parts = append(parts, []string{s.Text})
	}
	for i := range out {
		out[i].Text = strings.Join(parts[i], " ")
	}
	return out
}
