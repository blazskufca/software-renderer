// Command cobertura converts a Go coverage profile on stdin to Cobertura XML on
// stdout, the format GitHub code coverage ingests.
//
// Off-the-shelf converters find each file through go/packages, which only sees
// the files of one build configuration. `make cover` merges builds with
// different build tags (render_legacy.go vs render_simd.go, scalar vs SIMD
// raster kernels), so this maps profile paths to files by trimming the module
// path instead.
//
// Usage: go run ./internal/tools/cobertura < coverage.out > coverage.xml
package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "cobertura:", err)
		os.Exit(1)
	}
}

// blockRE matches a profile block: file:startLine.startCol,endLine.endCol numStmt count
var blockRE = regexp.MustCompile(`^(.+):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$`)

func run(in io.Reader, out io.Writer) error {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Path == "" {
		return errors.New("no module path in build info; run with `go run` from the module")
	}
	modPrefix := info.Main.Path + "/"
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		return errors.New("empty profile")
	}
	mode, ok := strings.CutPrefix(sc.Text(), "mode: ")
	if !ok {
		return fmt.Errorf("profile must start with a mode line, got %q", sc.Text())
	}

	// Hits per line per file. A line is hit if any block on it ran, as
	// gocover-cobertura does.
	files := map[string]map[int]int64{}
	for sc.Scan() {
		m := blockRE.FindStringSubmatch(sc.Text())
		if m == nil {
			return fmt.Errorf("malformed profile line %q", sc.Text())
		}
		file, ok := strings.CutPrefix(m[1], modPrefix)
		if !ok {
			return fmt.Errorf("%s is outside module %s", m[1], info.Main.Path)
		}
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		count, _ := strconv.ParseInt(m[4], 10, 64)
		lines := files[file]
		if lines == nil {
			lines = map[int]int64{}
			files[file] = lines
		}
		for l := start; l <= end; l++ {
			if mode == "set" {
				lines[l] = max(lines[l], count)
			} else {
				lines[l] += count
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}

	report := coverage{Timestamp: time.Now().UnixMilli(), Sources: []string{root}}
	var total tally
	byDir := map[string][]class{}
	dirTally := map[string]*tally{}
	for _, file := range slices.Sorted(maps.Keys(files)) {
		var t tally
		c := class{Name: file, Filename: file}
		for _, n := range slices.Sorted(maps.Keys(files[file])) {
			hits := files[file][n]
			c.Lines = append(c.Lines, line{Number: n, Hits: hits})
			t.add(hits)
		}
		c.LineRate = t.rate()
		dir := path.Dir(file)
		byDir[dir] = append(byDir[dir], c)
		if dirTally[dir] == nil {
			dirTally[dir] = &tally{}
		}
		dirTally[dir].merge(t)
		total.merge(t)
	}
	for _, dir := range slices.Sorted(maps.Keys(byDir)) {
		report.Packages = append(report.Packages, pkg{
			Name:     modPrefix + dir,
			LineRate: dirTally[dir].rate(),
			Classes:  byDir[dir],
		})
	}
	report.LineRate = total.rate()
	report.LinesCovered = total.covered
	report.LinesValid = total.valid

	if _, err := io.WriteString(out, xml.Header+
		`<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">`+"\n"); err != nil {
		return err
	}
	enc := xml.NewEncoder(out)
	enc.Indent("", "  ")
	if err := enc.Encode(report); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}

type tally struct{ covered, valid int }

func (t *tally) add(hits int64) {
	t.valid++
	if hits > 0 {
		t.covered++
	}
}

func (t *tally) merge(o tally) {
	t.covered += o.covered
	t.valid += o.valid
}

func (t tally) rate() float64 {
	if t.valid == 0 {
		return 0
	}
	return float64(t.covered) / float64(t.valid)
}

type coverage struct {
	XMLName         xml.Name `xml:"coverage"`
	LineRate        float64  `xml:"line-rate,attr"`
	BranchRate      float64  `xml:"branch-rate,attr"`
	Version         string   `xml:"version,attr"`
	Timestamp       int64    `xml:"timestamp,attr"`
	LinesCovered    int      `xml:"lines-covered,attr"`
	LinesValid      int      `xml:"lines-valid,attr"`
	BranchesCovered int      `xml:"branches-covered,attr"`
	BranchesValid   int      `xml:"branches-valid,attr"`
	Complexity      float64  `xml:"complexity,attr"`
	Sources         []string `xml:"sources>source"`
	Packages        []pkg    `xml:"packages>package"`
}

type pkg struct {
	Name       string  `xml:"name,attr"`
	LineRate   float64 `xml:"line-rate,attr"`
	BranchRate float64 `xml:"branch-rate,attr"`
	Complexity float64 `xml:"complexity,attr"`
	Classes    []class `xml:"classes>class"`
}

type class struct {
	Name       string   `xml:"name,attr"`
	Filename   string   `xml:"filename,attr"`
	LineRate   float64  `xml:"line-rate,attr"`
	BranchRate float64  `xml:"branch-rate,attr"`
	Complexity float64  `xml:"complexity,attr"`
	Methods    struct{} `xml:"methods"`
	Lines      []line   `xml:"lines>line"`
}

type line struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}
