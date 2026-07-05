package tui

import (
	"fmt"
	"strings"
)

const (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Green  = "\033[32m"
	Cyan   = "\033[36m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
)

type SelectOption struct {
	Label string
	Value string
}

func Select(title string, options []SelectOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options")
	}
	fmt.Printf("\n%s%s%s\n", Bold, title, Reset)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt.Label)
	}
	fmt.Printf("Choice [1-%d]: ", len(options))
	var n int
	fmt.Scanf("%d\n", &n)
	if n < 1 || n > len(options) { n = 1 }
	return options[n-1].Value, nil
}

func StatusBox(title string, lines []string) {
	width := 52
	fmt.Println()
	fmt.Printf("  %s┌%s┐%s\n", Green, strings.Repeat("─", width), Reset)
	fmt.Printf("  %s│%s %s%-*s%s %s│%s\n", Green, Reset, Bold, width-2, title, Reset, Green, Reset)
	fmt.Printf("  %s├%s%s%s┤%s\n", Green, Reset, strings.Repeat("─", width), Green, Reset)
	for _, line := range lines {
		fmt.Printf("  %s│%s %-*s %s│%s\n", Green, Reset, width-2, line, Green, Reset)
	}
	fmt.Printf("  %s└%s┘%s\n", Green, strings.Repeat("─", width), Reset)
}

func Table(headers []string, rows [][]string) {
	if len(rows) == 0 { return }
	widths := make([]int, len(headers))
	for i, h := range headers { widths[i] = len(h) }
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] { widths[i] = len(cell) }
		}
	}
	for i := range widths { if widths[i] > 30 { widths[i] = 30 } }
	sep := "  "
	fmt.Println()
	// Header
	line := ""
	for i, h := range headers {
		if i == 0 { line = fmt.Sprintf("%s%-*s%s", Bold, widths[i], h, Reset) } else { line += fmt.Sprintf("%s%-*s%s", sep, widths[i], h, Reset) }
	}
	fmt.Println(line)
	// Separator
	line = ""
	for i := range headers {
		l := strings.Repeat("\u2500", widths[i])
		if i == 0 { line = fmt.Sprintf("%s%s%s", Dim, l, Reset) } else { line += fmt.Sprintf("%s%s%s%s", sep, Dim, l, Reset) }
	}
	fmt.Println(line)
	// Rows
	for _, row := range rows {
		line = ""
		for i, cell := range row {
			v := cell
			if len(v) > widths[i] { v = v[:widths[i]-1] + "\u2026" }
			if i == 0 { line = fmt.Sprintf("%-*s", widths[i], v) } else { line += fmt.Sprintf("%s%-*s", sep, widths[i], v) }

		}
		fmt.Println(line)
	}
}
