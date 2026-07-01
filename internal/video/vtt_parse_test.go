package video

import "testing"

func TestParseVTTTimestamp(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"00:00:01.500", 1.5, true},
		{"01:02:03.250", 3723.25, true},
		{"02:03.100", 123.1, true}, // no hours
		{"garbage", 0, false},
		{"00:00:01", 0, false}, // missing millis
	}
	for _, c := range cases {
		got, ok := parseVTTTimestamp(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseVTTTimestamp(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestExtractSpeaker(t *testing.T) {
	cases := []struct {
		in, speaker, cleaned string
	}{
		{"<v Alice>Hello", "Alice", "Hello"},
		{"<v Martha Helena>Sí</v>", "Martha Helena", "Sí"},
		{"Bob: hi there", "Bob", "hi there"},
		{"Dr. Smith: point: two", "Dr. Smith", "point: two"}, // split first ": " only
		{"no speaker at all", "", "no speaker at all"},
	}
	for _, c := range cases {
		sp, cl := extractSpeaker(c.in)
		if sp != c.speaker || cl != c.cleaned {
			t.Errorf("extractSpeaker(%q) = %q,%q want %q,%q", c.in, sp, cl, c.speaker, c.cleaned)
		}
	}
}

func TestParseVTT_Teams(t *testing.T) {
	raw := "\xef\xbb\xbfWEBVTT\r\n\r\n" +
		"96c14169-de06-4080-b9fe-f573f75eb719/91-0\r\n" +
		"00:00:04.409 --> 00:00:09.359\r\n" +
		"<v Alice>Q3 numbers &amp; targets</v>\r\n\r\n"
	segs, err := parseVTT([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("want 1 segment, got %d", len(segs))
	}
	s := segs[0]
	if s.Speaker != "Alice" || s.Text != "Q3 numbers & targets" || s.Start != 4.409 {
		t.Errorf("got %+v", s)
	}
}

func TestParseVTT_Zoom(t *testing.T) {
	raw := "WEBVTT\n\n" +
		"1\n00:00:00.050 --> 00:00:01.790\nSrijani Ghosh: Hi!\n\n" +
		"2\n00:00:02.070 --> 00:00:04.050\nConor Healy: Get this party started.\n\n"
	segs, err := parseVTT([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 2 || segs[0].Speaker != "Srijani Ghosh" || segs[0].Text != "Hi!" {
		t.Fatalf("got %+v", segs)
	}
}

func TestParseVTT_NoSpeaker(t *testing.T) {
	raw := "WEBVTT\n\n00:01:08.000 --> 00:01:09.000\nThis doesn't.\n\n"
	segs, err := parseVTT([]byte(raw))
	if err != nil || len(segs) != 1 || segs[0].Speaker != "" || segs[0].Text != "This doesn't." {
		t.Fatalf("got %+v err %v", segs, err)
	}
}

func TestParseVTT_MultiLineCue(t *testing.T) {
	raw := "WEBVTT\n\n00:00:00.000 --> 00:00:02.000\n<v Al>line one\nline two\n\n"
	segs, _ := parseVTT([]byte(raw))
	if len(segs) != 1 || segs[0].Text != "line one\nline two" {
		t.Fatalf("got %+v", segs)
	}
}

func TestParseVTT_SkipsNoteAndStyle(t *testing.T) {
	raw := "WEBVTT\n\nNOTE this is a comment\n\nSTYLE\n::cue { color: white }\n\n" +
		"00:00:00.000 --> 00:00:01.000\nHello\n\n"
	segs, err := parseVTT([]byte(raw))
	if err != nil || len(segs) != 1 || segs[0].Text != "Hello" {
		t.Fatalf("got %+v err %v", segs, err)
	}
}

func TestParseVTT_SkipsTagOnlyPayload(t *testing.T) {
	raw := "WEBVTT\n\n" +
		"00:00:00.000 --> 00:00:01.000\n<c></c>\n\n" +
		"00:00:01.000 --> 00:00:02.000\nHello\n\n"
	segs, err := parseVTT([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 1 || segs[0].Text != "Hello" {
		t.Fatalf("want 1 segment with text %q, got %+v", "Hello", segs)
	}
}

func TestParseVTT_Errors(t *testing.T) {
	if _, err := parseVTT([]byte("not a vtt file")); err == nil {
		t.Error("want error for missing signature")
	}
	if _, err := parseVTT([]byte("WEBVTT\n\n")); err == nil {
		t.Error("want error for zero cues")
	}
}

func TestMergeSegments(t *testing.T) {
	in := []TranscriptSegment{
		{Start: 0, End: 1, Text: "Um", Speaker: "A"},
		{Start: 1.5, End: 2, Text: "yeah", Speaker: "A"},   // gap 0.5s, same speaker -> merge
		{Start: 2, End: 3, Text: "ok", Speaker: "B"},       // diff speaker -> new
		{Start: 10, End: 11, Text: "later", Speaker: "B"},  // gap 7s -> new
	}
	got := mergeSegments(in)
	if len(got) != 3 {
		t.Fatalf("want 3 merged, got %d: %+v", len(got), got)
	}
	if got[0].Text != "Um yeah" || got[0].End != 2 {
		t.Errorf("first merge wrong: %+v", got[0])
	}
}

func TestMergeSegments_EmptySpeakerMerges(t *testing.T) {
	in := []TranscriptSegment{
		{Start: 0, End: 1, Text: "a"},
		{Start: 1, End: 2, Text: "b"},
	}
	got := mergeSegments(in)
	if len(got) != 1 || got[0].Text != "a b" {
		t.Fatalf("got %+v", got)
	}
}
