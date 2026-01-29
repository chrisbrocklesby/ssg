package ssg

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reTemplateRender = regexp.MustCompile(`^([^:]+): render: template: ([^:]+):(\d+): (.+)$`)
	reTemplateParse  = regexp.MustCompile(`^([^:]+): template: ([^:]+):(\d+): (.+)$`)
	reTemplateBare   = regexp.MustCompile(`^template: ([^:]+):(\d+): (.+)$`)
)

func humanizeError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if out, ok := humanizeTemplateErrorString(s); ok {
		return out
	}
	return s
}

// HumanizeError converts internal/ssg errors into a more human-friendly message.
// It is intended for CLI/devserver presentation only.
func HumanizeError(err error) string {
	return humanizeError(err)
}

func humanizeTemplateErrorString(s string) (string, bool) {
	for _, re := range []*regexp.Regexp{reTemplateRender, reTemplateParse} {
		m := re.FindStringSubmatch(s)
		if len(m) == 0 {
			continue
		}
		// m[1]=file, m[2]=template name, m[3]=line, m[4]=message
		out := formatTemplateErr(m[1], m[2], m[3], m[4])
		return out, true
	}

	if m := reTemplateBare.FindStringSubmatch(s); len(m) > 0 {
		// m[1]=template name, m[2]=line, m[3]=message
		out := formatTemplateErr("", m[1], m[2], m[3])
		return out, true
	}

	return "", false
}

func formatTemplateErr(file, tpl, line, msg string) string {
	file = strings.TrimSpace(file)
	tpl = strings.TrimSpace(tpl)
	line = strings.TrimSpace(line)
	msg = strings.TrimSpace(msg)

	var b strings.Builder
	if file != "" {
		b.WriteString(file)
		b.WriteString(" ")
	}

	if tpl == "" || tpl == "ssg" {
		fmt.Fprintf(&b, "template line %s: %s", line, msg)
	} else {
		fmt.Fprintf(&b, "template %q line %s: %s", tpl, line, msg)
	}

	out := strings.TrimSpace(b.String())
	if out != "" {
		switch out[len(out)-1] {
		case '.', '!', '?':
			// keep
		default:
			out += "."
		}
	}
	return out
}
