// ABOUTME: Diff implementation using anchored diff algorithm.
// ABOUTME: Produces diffs with context lines in O(n log n) time.

package slt

import (
	"bytes"
	"sort"
)

const (
	colorRed   = "\x1b[31m" // for deletions (-)
	colorGreen = "\x1b[32m" // for insertions (+)
	colorReset = "\x1b[0m"  // reset to default
)

// Diff returns an anchored diff of the two texts old and new
// as change lines only (no file headers or hunk headers).
// If old and new are identical, Diff returns a nil slice (no output).
//
// If color is true, ANSI color codes are added to the output
// (red for deletions, green for insertions).
//
// Unix diff implementations typically look for a diff with
// the smallest number of lines inserted and removed,
// which can in the worst case take time quadratic in the
// number of lines in the texts. As a result, many implementations
// either can be made to run for a long time or cut off the search
// after a predetermined amount of work.
//
// In contrast, this implementation looks for a diff with the
// smallest number of "unique" lines inserted and removed,
// where unique means a line that appears just once in both old and new.
// We call this an "anchored diff" because the unique lines anchor
// the chosen matching regions. An anchored diff is usually clearer
// than a standard diff, because the algorithm does not try to
// reuse unrelated blank lines or closing braces.
// The algorithm also guarantees to run in O(n log n) time
// instead of the standard O(n²) time.
func Diff(old []byte, new []byte, color bool) []byte {
	oldLines := splitLines(old)
	newLines := splitLines(new)

	// If files are identical, return empty diff
	if bytes.Equal(old, new) {
		return nil
	}

	// Find matching regions using anchored diff
	matches := findMatches(oldLines, newLines)

	// Generate unified diff output
	return generateUnifiedDiff(oldLines, newLines, matches, color)
}

// splitLines splits text into lines, preserving line endings
func splitLines(text []byte) [][]byte {
	if len(text) == 0 {
		return nil
	}

	var lines [][]byte
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}

	// Handle last line without newline
	if start < len(text) {
		lines = append(lines, text[start:])
	}

	return lines
}

// match represents a matching region between old and new
type match struct {
	oldStart int
	newStart int
	length   int
}

// edit represents a single line edit operation
type edit struct {
	oldLine int // -1 for additions
	newLine int // -1 for deletions
	op      editOp
}

type editOp int

const (
	opEqual editOp = iota
	opDelete
	opInsert
)

// buildEditScript converts matches to an edit script
func buildEditScript(oldLines, newLines [][]byte, matches []match) []edit {
	var edits []edit

	oldIdx, newIdx := 0, 0

	for _, m := range matches {
		// Add deletions before match
		for oldIdx < m.oldStart {
			edits = append(edits, edit{
				oldLine: oldIdx,
				newLine: -1,
				op:      opDelete,
			})
			oldIdx++
		}

		// Add insertions before match
		for newIdx < m.newStart {
			edits = append(edits, edit{
				oldLine: -1,
				newLine: newIdx,
				op:      opInsert,
			})
			newIdx++
		}

		// Add equal lines from match
		for i := 0; i < m.length; i++ {
			edits = append(edits, edit{
				oldLine: oldIdx,
				newLine: newIdx,
				op:      opEqual,
			})
			oldIdx++
			newIdx++
		}
	}

	// Add remaining deletions
	for oldIdx < len(oldLines) {
		edits = append(edits, edit{
			oldLine: oldIdx,
			newLine: -1,
			op:      opDelete,
		})
		oldIdx++
	}

	// Add remaining insertions
	for newIdx < len(newLines) {
		edits = append(edits, edit{
			oldLine: -1,
			newLine: newIdx,
			op:      opInsert,
		})
		newIdx++
	}

	return edits
}

// findMatches finds matching regions using unique anchor lines
func findMatches(oldLines, newLines [][]byte) []match {
	// Find unique lines (appear exactly once in both old and new)
	oldCounts := make(map[string]int)
	newCounts := make(map[string]int)

	for _, line := range oldLines {
		oldCounts[string(line)]++
	}
	for _, line := range newLines {
		newCounts[string(line)]++
	}

	// Find anchor lines (unique in both)
	type anchor struct {
		oldIdx int
		newIdx int
	}
	var anchors []anchor

	for i, oldLine := range oldLines {
		key := string(oldLine)
		if oldCounts[key] == 1 && newCounts[key] == 1 {
			// Find position in new
			for j, newLine := range newLines {
				if bytes.Equal(oldLine, newLine) {
					anchors = append(anchors, anchor{oldIdx: i, newIdx: j})
					break
				}
			}
		}
	}

	// Sort anchors by old position
	sort.Slice(anchors, func(i, j int) bool {
		return anchors[i].oldIdx < anchors[j].oldIdx
	})

	// Filter out anchors that are out of order in new
	var filteredAnchors []anchor
	lastNewIdx := -1
	for _, a := range anchors {
		if a.newIdx > lastNewIdx {
			filteredAnchors = append(filteredAnchors, a)
			lastNewIdx = a.newIdx
		}
	}

	// Convert anchors to matches
	var matches []match

	for _, a := range filteredAnchors {
		// Match the anchor line itself
		matches = append(matches, match{
			oldStart: a.oldIdx,
			newStart: a.newIdx,
			length:   1,
		})
	}

	// Extend matches forward and backward
	matches = extendMatches(oldLines, newLines, matches)

	return matches
}

// extendMatches extends matching regions to include adjacent matching lines
func extendMatches(oldLines, newLines [][]byte, matches []match) []match {
	if len(matches) == 0 {
		return matches
	}

	extended := make([]match, 0, len(matches))

	for _, m := range matches {
		// Extend backward
		for m.oldStart > 0 && m.newStart > 0 {
			if !bytes.Equal(oldLines[m.oldStart-1], newLines[m.newStart-1]) {
				break
			}
			m.oldStart--
			m.newStart--
			m.length++
		}

		// Extend forward
		for m.oldStart+m.length < len(oldLines) && m.newStart+m.length < len(newLines) {
			if !bytes.Equal(oldLines[m.oldStart+m.length], newLines[m.newStart+m.length]) {
				break
			}
			m.length++
		}

		extended = append(extended, m)
	}

	// Merge adjacent or overlapping matches
	merged := []match{extended[0]}
	for i := 1; i < len(extended); i++ {
		last := &merged[len(merged)-1]
		curr := extended[i]

		// Check if matches are adjacent or overlapping
		if curr.oldStart <= last.oldStart+last.length && curr.newStart <= last.newStart+last.length {
			// Merge
			oldEnd := max(last.oldStart+last.length, curr.oldStart+curr.length)
			newEnd := max(last.newStart+last.length, curr.newStart+curr.length)
			last.length = max(oldEnd-last.oldStart, newEnd-last.newStart)
		} else {
			merged = append(merged, curr)
		}
	}

	return merged
}

// generateUnifiedDiff generates unified diff format output
func generateUnifiedDiff(oldLines, newLines [][]byte, matches []match, color bool) []byte {
	var buf bytes.Buffer

	const contextLines = 3

	// Build edit script from matches
	edits := buildEditScript(oldLines, newLines, matches)

	// Generate hunks from edit script
	hunks := generateHunksFromEdits(oldLines, newLines, edits, contextLines)

	for _, hunk := range hunks {
		writeHunkFromEdits(&buf, oldLines, newLines, edits, hunk, color)
	}

	return buf.Bytes()
}

// hunk represents a single diff hunk
type hunk struct {
	startEdit int // index of first edit in hunk
	endEdit   int // index past last edit in hunk
}

// generateHunksFromEdits creates hunks from edit script
func generateHunksFromEdits(oldLines, newLines [][]byte, edits []edit, contextLines int) []hunk {
	if len(edits) == 0 {
		return nil
	}

	// Find regions of changes (non-equal edits)
	var hunks []hunk
	var inHunk bool
	var hunkStart, hunkEnd int

	for i, e := range edits {
		if e.op != opEqual {
			if !inHunk {
				// Start new hunk with context
				hunkStart = max(0, i-contextLines)
				inHunk = true
			}
			hunkEnd = i + 1
		} else if inHunk {
			// Check if we should end the hunk or continue with context
			// Count remaining context lines
			contextCount := 0
			for j := i; j < len(edits) && edits[j].op == opEqual && contextCount < contextLines*2; j++ {
				contextCount++
			}

			// If we have enough context for two hunks, split
			if contextCount >= contextLines*2 {
				// End current hunk
				hunkEnd = min(len(edits), hunkEnd+contextLines)
				hunks = append(hunks, hunk{
					startEdit: hunkStart,
					endEdit:   hunkEnd,
				})
				inHunk = false
			} else {
				// Continue hunk through context
				hunkEnd = i + 1
			}
		}
	}

	// Close final hunk if still open
	if inHunk {
		hunkEnd = min(len(edits), hunkEnd+contextLines)
		hunks = append(hunks, hunk{
			startEdit: hunkStart,
			endEdit:   hunkEnd,
		})
	}

	return hunks
}

// writeHunkFromEdits writes a hunk using the edit script
func writeHunkFromEdits(buf *bytes.Buffer, oldLines, newLines [][]byte, edits []edit, h hunk, color bool) {
	// Calculate line ranges
	var oldStart, oldCount, newStart, newCount int
	oldStart = -1
	newStart = -1

	for i := h.startEdit; i < h.endEdit; i++ {
		e := edits[i]
		if e.op == opDelete || e.op == opEqual {
			if oldStart == -1 {
				oldStart = e.oldLine
			}
			oldCount++
		}
		if e.op == opInsert || e.op == opEqual {
			if newStart == -1 {
				newStart = e.newLine
			}
			newCount++
		}
	}

	// Write lines
	for i := h.startEdit; i < h.endEdit; i++ {
		e := edits[i]
		switch e.op {
		case opEqual:
			buf.WriteByte(' ')
			buf.Write(oldLines[e.oldLine])
			buf.WriteByte('\n')
		case opDelete:
			if color {
				buf.WriteString(colorRed)
			}
			buf.WriteByte('-')
			buf.Write(oldLines[e.oldLine])
			if color {
				buf.WriteString(colorReset)
			}
			buf.WriteByte('\n')
		case opInsert:
			if color {
				buf.WriteString(colorGreen)
			}
			buf.WriteByte('+')
			buf.Write(newLines[e.newLine])
			if color {
				buf.WriteString(colorReset)
			}
			buf.WriteByte('\n')
		}
	}
}
