package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// progressReader wraps an io.Reader and prints live upload progress to stderr.
// When active is false it is a transparent pass-through with no output.
type progressReader struct {
	r       io.Reader
	total   int64
	read    int64
	label   string
	active  bool
	lastLen int
}

func newProgressReader(r io.Reader, total int64, label string, active bool) *progressReader {
	return &progressReader{r: r, total: total, label: label, active: active}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if pr.active && n > 0 {
		pr.read += int64(n)
		printProgress(pr.label, pr.read, pr.total, &pr.lastLen)
	}
	return n, err
}

func (pr *progressReader) finish() {
	if pr.active {
		fmt.Fprintln(os.Stderr)
	}
}

// progressWriter wraps an io.Writer and prints live download progress to stderr.
type progressWriter struct {
	w       io.Writer
	total   int64
	written int64
	label   string
	active  bool
	lastLen int
}

func newProgressWriter(w io.Writer, total int64, label string, active bool) *progressWriter {
	return &progressWriter{w: w, total: total, label: label, active: active}
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.w.Write(p)
	if pw.active && n > 0 {
		pw.written += int64(n)
		printProgress(pw.label, pw.written, pw.total, &pw.lastLen)
	}
	return n, err
}

func (pw *progressWriter) finish() {
	if pw.active {
		fmt.Fprintln(os.Stderr)
	}
}

func printProgress(label string, done, total int64, lastLen *int) {
	lbl := truncLabel(label, 36)
	var line string
	if total > 0 {
		pct := float64(done) / float64(total) * 100
		if pct > 100 {
			pct = 100
		}
		bar := progressBar(int(pct), 24)
		line = fmt.Sprintf("\r  %-36s  %8s / %-8s  [%s]  %3.0f%%",
			lbl, formatSize(done), formatSize(total), bar, pct)
	} else {
		line = fmt.Sprintf("\r  %-36s  %s", lbl, formatSize(done))
	}
	// Pad to erase any longer previous line.
	if len(line) < *lastLen {
		line += strings.Repeat(" ", *lastLen-len(line))
	}
	*lastLen = len(line)
	fmt.Fprint(os.Stderr, line)
}

// progressBar returns a fixed-width ASCII progress bar string.
func progressBar(pct, width int) string {
	filled := pct * width / 100
	if filled >= width {
		return strings.Repeat("=", width)
	}
	if filled == 0 {
		return strings.Repeat(" ", width)
	}
	return strings.Repeat("=", filled-1) + ">" + strings.Repeat(" ", width-filled)
}

// truncLabel shortens s to limit runes, prefixing with "…" if needed.
func truncLabel(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return "…" + s[len(s)-(limit-1):]
}
