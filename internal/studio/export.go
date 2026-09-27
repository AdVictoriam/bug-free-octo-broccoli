package studio

import (
	"fmt"
	"html"
	"strings"
)

func md(s string) string {
	return strings.NewReplacer("\\", "\\\\", "|", "\\|", "\n", " ").Replace(html.EscapeString(s))
}
func Markdown(s Script) (string, string) {
	var va, ed strings.Builder
	fmt.Fprintf(&va, "# %s\n\n## Voice actor script\n\nReading voices\n\n| Voice | Role | Performance |\n|---|---|---|\n", md(s.Title))
	for _, v := range s.Voices {
		fmt.Fprintf(&va, "| %s | %s | %s |\n", md(v.Name), md(v.Role), md(v.Delivery))
	}
	va.WriteString("\n---\n\n")
	for _, l := range s.Lines {
		star := ""
		if l.SecondTake {
			star = " ★"
		}
		fmt.Fprintf(&va, "**%s%s · %s**  \n<span style=\"color:#8242BB\">%s</span>  \n%s\n\n", lineLabel(l.ID), star, md(l.Speaker), md(l.Delivery), md(l.Text))
	}
	fmt.Fprintf(&ed, "# %s\n\n## Editor script\n\n### Thumbnail\n\n**Left:** %s  \n**Right:** %s  \n**Central evidence:** %s  \n**Text:** %s\n\n### Tone\n\n%s\n\n", md(s.Title), md(s.Thumbnail.Left), md(s.Thumbnail.Right), md(s.Thumbnail.Evidence), md(s.Thumbnail.Text), md(s.Tone))
	if s.Disclosure != "" {
		fmt.Fprintf(&ed, "<span style=\"color:#B43B40\">Disclosure: %s</span>\n\n", md(s.Disclosure))
	}
	ed.WriteString("### Privacy and recreation rules\n\n")
	for _, v := range s.Privacy {
		fmt.Fprintf(&ed, "- <span style=\"color:#B43B40\">%s</span>\n", md(v))
	}
	ed.WriteString("\n### Assets\n\n| Asset | Build specification | Provenance |\n|---|---|---|\n")
	for _, v := range s.Assets {
		fmt.Fprintf(&ed, "| %s | %s | %s |\n", md(v.Name), md(v.Description), md(v.Provenance))
	}
	ed.WriteString("\n### Running gags\n\n")
	for _, v := range s.RunningGags {
		fmt.Fprintf(&ed, "- %s\n", md(v))
	}
	ed.WriteString("\n## Line-synced edit\n\n")
	for _, q := range s.Cues {
		fmt.Fprintf(&ed, "### %s\n\n", lineLabel(q.LineID))
		if q.Gameplay != "" {
			fmt.Fprintf(&ed, "<span style=\"color:#2465A7\">Gameplay / general: %s</span>\n\n", md(q.Gameplay))
		}
		if q.Edit != "" {
			fmt.Fprintf(&ed, "<span style=\"color:#B43B40\">Edit: %s</span>\n\n", md(q.Edit))
		}
		if q.Sound != "" {
			fmt.Fprintf(&ed, "<span style=\"color:#B43B40\">Sound: %s</span>\n\n", md(q.Sound))
		}
		if q.HoldSeconds > 0 {
			fmt.Fprintf(&ed, "<span style=\"color:#B43B40\">Additional non-spoken hold: %.1f seconds.</span>\n\n", q.HoldSeconds)
		}
	}
	return va.String(), ed.String()
}
