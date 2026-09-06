package video

import (
	"strings"
	"testing"
)

// maxChainDepth mirrors how libavutil/eval.c measures an expression: every
// binary operator adds one level over its deeper operand, and parentheses only
// group. The parser rejects anything deeper than MAX_DEPTH (100).
//
// It walks the expression once, tracking for every open paren how many '+'
// operands it has seen, and returns the deepest operator tree.
func maxChainDepth(expr string) int {
	type frame struct{ plus, childMax int }
	stack := []frame{{}}
	for _, c := range expr {
		top := &stack[len(stack)-1]
		switch c {
		case '(':
			stack = append(stack, frame{})
		case ')':
			done := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			d := done.plus + done.childMax
			parent := &stack[len(stack)-1]
			if d > parent.childMax {
				parent.childMax = d
			}
		case '+':
			top.plus++
		}
	}
	root := stack[0]
	return root.plus + root.childMax
}

func TestSegmentExpressionsStayWithinFFmpegDepthLimit(t *testing.T) {
	segments := make([]segmentRange, maxSegments)
	for i := range segments {
		segments[i] = segmentRange{Start: float64(i), End: float64(i) + 0.5}
	}
	args := buildRemoveSegmentsArgs("in.mp4", "out.mp4", "video/mp4", segments, true)
	var filter string
	for i, a := range args {
		if a == "-filter_complex" {
			filter = args[i+1]
		}
	}
	if filter == "" {
		t.Fatal("no -filter_complex in args")
	}
	// Both the select and the setpts expressions carry one term per segment.
	for _, expr := range []string{"select='", "setpts='PTS-"} {
		start := strings.Index(filter, expr)
		if start < 0 {
			t.Fatalf("expression %q not found in filter", expr)
		}
		rest := filter[start+len(expr):]
		end := strings.Index(rest, "'")
		if expr == "setpts='PTS-" {
			end = strings.Index(rest, "/TB")
		}
		body := rest[:end]
		if d := maxChainDepth(body); d > 100 {
			t.Errorf("%s expression nests %d deep for %d segments; ffmpeg rejects more than 100", expr, d, maxSegments)
		}
	}
}
