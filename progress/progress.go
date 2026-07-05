package progress

import (
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ANSIup    = "\033[A"
	ANSIreset = "\033[0m"
	ANSIbold  = "\033[1m"
	ANSIgreen = "\033[32m"
	ANSIyellow = "\033[33m"
	ANSIdim   = "\033[2m"
	ANSIcyan  = "\033[36m"
	ANSIred   = "\033[31m"
)

type Layer struct {
	name      string
	status    string
	startTime time.Time
	progress  float64
}

type Display struct {
	layers   []*Layer
	msg      string
	msgLines int
	rendered bool
	barWidth int
}

func New() *Display {
	return &Display{barWidth: 30}
}

func (d *Display) AddLayer(name string) *Layer {
	l := &Layer{
		name:      name,
		status:    "waiting",
		startTime: time.Now(),
	}
	d.layers = append(d.layers, l)
	return l
}

func (l *Layer) Start()     { l.status = "backing"; l.startTime = time.Now(); l.progress = 0 }
func (l *Layer) Done()      { l.status = "done"; l.progress = 1.0 }
func (l *Layer) Fail()      { l.status = "error" }
func (l *Layer) Skip()      { l.status = "skipped"; l.progress = 1.0 }
func (l *Layer) SetProgress(p float64) { l.progress = p }

func (d *Display) Render() {
	total := len(d.layers) + d.msgLines
	if d.rendered && total > 0 {
		fmt.Printf("\033[%dA", total)
	}
	lines := 0
	for _, l := range d.layers {
		fmt.Print(d.renderLayer(l))
		lines++
	}
	if d.msg != "" {
		fmt.Print("\033[2K" + d.msg + "\n")
		lines++
	}
	d.msgLines = lines - len(d.layers)
	if d.msgLines < 0 {
		d.msgLines = 0
	}
	d.rendered = true
}

func (d *Display) SetMessage(msg string) { d.msg = msg }

func (d *Display) renderLayer(l *Layer) string {
	elapsed := time.Since(l.startTime)
	secs := int(elapsed.Seconds())
	elapsedStr := fmt.Sprintf("%ds", secs)
	if secs < 1 {
		elapsedStr = "<1s"
	}

	var icon, color, statusText string
	switch l.status {
	case "waiting":
		icon, color, statusText = " ", ANSIdim, "Queued"
	case "backing":
		icon, color, statusText = ">", ANSIcyan, "Backing up"
	case "extracting":
		icon, color, statusText = "-", ANSIyellow, "Extracting"
	case "done":
		icon, color, statusText = "\u2714", ANSIgreen, "Backup done"
	case "skipped":
		icon, color, statusText = "\u00b7", ANSIdim, "Not found"
	case "error":
		icon, color, statusText = "\u2718", ANSIred, "Error"
	}

	name := l.name
	if utf8.RuneCountInString(name) > 12 {
		runes := []rune(name)
		name = string(runes[:11]) + "\u2026"
	}

	bar := d.renderBar(l.progress)
	line := fmt.Sprintf(" %s %s%-12s%s %s %s%-12s%s %s%s%s",
		icon, ANSIbold, name, ANSIreset,
		bar,
		color, statusText, ANSIreset,
		ANSIdim, elapsedStr, ANSIreset,
	)
	return "\033[2K" + line + "\n"
}

func (d *Display) renderBar(pct float64) string {
	if pct < 0 { pct = 0 }
	if pct > 1 { pct = 1 }
	filled := int(math.Round(pct * float64(d.barWidth)))
	empty := d.barWidth - filled
	var sb strings.Builder
	sb.WriteString(ANSIcyan)
	sb.WriteString("[")
	sb.WriteString(strings.Repeat("=", filled))
	if filled < d.barWidth {
		sb.WriteString(">")
		empty--
	}
	sb.WriteString(strings.Repeat(" ", empty))
	sb.WriteString("]")
	sb.WriteString(ANSIreset)
	sb.WriteString(fmt.Sprintf(" %3.0f%%", pct*100))
	return sb.String()
}

func (d *Display) Done() {
	for _, l := range d.layers {
		if l.status == "backing" || l.status == "extracting" {
			l.status = "done"
			l.progress = 1.0
		}
	}
	d.Render()
	fmt.Println()
}

func (d *Display) Header() {
	fmt.Printf("\n%s%sBackup from minibox%s\n", ANSIbold, ANSIgreen, ANSIreset)
	fmt.Printf("%s-----  layers: [mysql, mongodb, pm2, nginx, ssl, git, cron, docker]%s\n\n", ANSIdim, ANSIreset)
}
