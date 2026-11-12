package export

import (
	"fmt"
	"strings"
)

// RenderBragMarkdown formats a BragReport as clean Markdown for performance reviews or Notion/GitHub.
func RenderBragMarkdown(report *BragReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Engineering Brag Document — %s\n\n", report.User.FullName)
	fmt.Fprintf(&b, "**Organization:** %s  \n", report.OrgName)
	fmt.Fprintf(&b, "**Email:** %s  \n", report.User.Email)
	fmt.Fprintf(&b, "**Report Period:** %s to %s  \n", report.StartDate.Format("2006-01-02"), report.EndDate.Format("2006-01-02"))
	fmt.Fprintf(&b, "**Generated:** %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04 UTC"))

	b.WriteString("## Summary Metrics\n\n")
	fmt.Fprintf(&b, "- **Total Tasks:** %d\n", report.Summary.TotalTasks)
	fmt.Fprintf(&b, "- **Completed:** %d (%.1f%%)\n", report.Summary.CompletedTasks, report.Summary.CompletionRate)
	fmt.Fprintf(&b, "- **In Progress:** %d\n", report.Summary.InProgressTasks)
	fmt.Fprintf(&b, "- **Features Shipped:** %d\n", report.Summary.FeaturesShipped)
	fmt.Fprintf(&b, "- **Bugs Resolved:** %d\n\n", report.Summary.BugsResolved)
	fmt.Fprintf(&b, "> **Key Insight:** %s\n\n", report.KeyInsight)

	b.WriteString("## Impact by Project\n\n")
	b.WriteString("| Project | Prefix | Total | Completed | Completion |\n")
	b.WriteString("|---|---|---:|---:|---:|\n")
	for _, p := range report.ProjectBreakdown {
		fmt.Fprintf(&b, "| %s | `%s` | %d | %d | %.1f%% |\n", p.ProjectName, p.Prefix, p.Total, p.Completed, p.CompletionRate)
	}
	b.WriteString("\n")

	b.WriteString("## Completed Accomplishments\n\n")
	if len(report.CompletedWork) == 0 {
		b.WriteString("_No completed tasks recorded in this period._\n\n")
	} else {
		for _, item := range report.CompletedWork {
			summary := strings.TrimSpace(item.WorkSummary)
			if summary == "" {
				summary = item.Description
			}
			fmt.Fprintf(&b, "### [%s] %s (`%s`)\n", item.Key, item.Title, item.TicketType)
			fmt.Fprintf(&b, "- **Project:** %s | **Priority:** %s | **Updated:** %s\n", item.ProjectName, item.Priority, item.UpdatedAt.Format("2006-01-02"))
			fmt.Fprintf(&b, "- **What Was Done:** %s\n\n", summary)
		}
	}

	if len(report.ActiveWork) > 0 {
		b.WriteString("## Active & In-Progress Work\n\n")
		for _, item := range report.ActiveWork {
			prog := "N/A"
			if item.ProgressPercent != nil {
				prog = fmt.Sprintf("%d%%", *item.ProgressPercent)
			}
			summary := strings.TrimSpace(item.WorkSummary)
			if summary == "" {
				summary = item.Description
			}
			fmt.Fprintf(&b, "### [%s] %s (`%s` — %s, %s)\n", item.Key, item.Title, item.TicketType, item.Status, prog)
			fmt.Fprintf(&b, "- **Progress Notes:** %s\n\n", summary)
		}
	}

	return b.String()
}

