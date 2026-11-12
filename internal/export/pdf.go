package export

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
)

// RenderBragPDF builds an executive Brag / Performance Report PDF inspired by modern HR/engineering scorecards.
func RenderBragPDF(report *BragReport) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AliasNbPages("")

	pdf.SetFooterFunc(func() {
		pdf.SetY(-14)
		pdf.SetDrawColor(226, 232, 240)
		pdf.Line(14, pdf.GetY(), 196, pdf.GetY())
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(19, 34, 71)
		pdf.CellFormat(45, 6, "Great work!", "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(95, 6, "Your contribution drives engineering excellence.", "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 6, fmt.Sprintf("%s  |  Page %d/{nb}", safePDFText(report.OrgName), pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()

	// 1. Top Navy Banner with Crimson Accent
	pdf.SetFillColor(19, 34, 71) // Deep Navy #132247
	pdf.Rect(0, 0, 210, 20, "F")
	pdf.SetFillColor(225, 29, 72) // Crimson #E11D48
	pdf.Polygon([]fpdf.PointType{
		{X: 192, Y: 0},
		{X: 210, Y: 0},
		{X: 210, Y: 20},
		{X: 180, Y: 20},
	}, "F")

	pdf.SetXY(14, 5)
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(95, 6, safePDFText(report.OrgName), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(203, 213, 225)
	pdf.CellFormat(78, 6, "Build a productive, accountable & high-impact team.", "", 1, "R", false, 0, "")

	// 2. Eyebrow, Personalized Greeting & Report Period Badge
	pdf.SetY(26)
	pdf.SetDrawColor(225, 29, 72)
	pdf.SetLineWidth(0.8)
	pdf.Line(14, 28, 22, 28)
	pdf.SetLineWidth(0.2)

	pdf.SetXY(24, 25.5)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.CellFormat(95, 5, "ENGINEERING BRAG & PERFORMANCE REPORT", "", 0, "L", false, 0, "")

	// Report Period Box on top-right
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(136, 24, 60, 18, 2, "1234", "FD")
	pdf.SetXY(139, 25.5)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(100, 116, 139)
	pdf.CellFormat(54, 4, "REPORT PERIOD", "", 1, "L", false, 0, "")
	pdf.SetX(139)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(15, 23, 42)
	periodStr := fmt.Sprintf("%s - %s", report.StartDate.Format("Jan 02, 2006"), report.EndDate.Format("Jan 02, 2006"))
	pdf.CellFormat(54, 5, periodStr, "", 1, "L", false, 0, "")
	pdf.SetX(139)
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(100, 116, 139)
	pdf.CellFormat(54, 4, "Generated: "+report.GeneratedAt.Format("Jan 02, 2006 15:04 UTC"), "", 1, "L", false, 0, "")

	// Greeting Line
	firstName := firstWord(report.User.FullName)
	pdf.SetXY(14, 31)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetTextColor(15, 23, 42)
	prefixTxt := "Brag Report, "
	wPrefix := pdf.GetStringWidth(prefixTxt)
	pdf.CellFormat(wPrefix, 8, prefixTxt, "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "BI", 18)
	pdf.SetTextColor(225, 29, 72)
	pdf.CellFormat(80, 8, safePDFText(firstName)+".", "", 1, "L", false, 0, "")

	pdf.SetXY(14, 40)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(100, 116, 139)
	pdf.CellFormat(120, 5, safePDFText(fmt.Sprintf("%s (%s) - Impact summary for the selected period.", report.User.FullName, report.User.Email)), "", 1, "L", false, 0, "")

	// 3. Five KPI Summary Cards
	pdf.SetY(49)
	drawKPICards(pdf, report)

	// 4. Work Breakdown Bar + Key Insight Box
	pdf.SetY(80)
	drawBreakdownAndInsight(pdf, report)

	// 5. Performance by Project Table
	pdf.SetY(118)
	drawProjectBreakdownTable(pdf, report)

	// 6. Completed Accomplishments & Active Work Tables
	pdf.Ln(6)
	drawWorkTable(pdf, "Completed Accomplishments", report.CompletedWork, true)

	if len(report.ActiveWork) > 0 {
		pdf.Ln(6)
		drawWorkTable(pdf, "Active & In-Progress Work", report.ActiveWork, false)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render pdf failed: %w", err)
	}
	return buf.Bytes(), nil
}

func drawKPICards(pdf *fpdf.Fpdf, report *BragReport) {
	type kpiCard struct {
		label string
		value string
		sub   string
		badge string
		r, g, b int
	}

	cards := []kpiCard{
		{"TOTAL TASKS", fmt.Sprintf("%d", report.Summary.TotalTasks), "Assigned in period", "100%", 79, 70, 229},
		{"COMPLETED", fmt.Sprintf("%d", report.Summary.CompletedTasks), "Shipped & verified", fmt.Sprintf("%.0f%%", report.Summary.CompletionRate), 16, 185, 129},
		{"IN PROGRESS", fmt.Sprintf("%d", report.Summary.InProgressTasks), "Active execution", fmt.Sprintf("%d hold", report.Summary.OnHoldTasks), 245, 158, 11},
		{"FEATURES", fmt.Sprintf("%d", report.Summary.FeaturesShipped), "New capabilities", fmt.Sprintf("%d bugs", report.Summary.BugsResolved), 14, 165, 233},
		{"DELIVERY RATE", fmt.Sprintf("%.1f%%", report.Summary.CompletionRate), "Period velocity", "Target", 225, 29, 72},
	}

	cardW := 34.4
	gap := 2.5
	startX := 14.0
	y := pdf.GetY()

	for i, c := range cards {
		x := startX + float64(i)*(cardW+gap)
		pdf.SetFillColor(255, 255, 255)
		pdf.SetDrawColor(226, 232, 240)
		pdf.RoundedRect(x, y, cardW, 26, 2, "1234", "FD")

		pdf.SetXY(x+3, y+2.5)
		pdf.SetFont("Helvetica", "B", 6.5)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(cardW-6, 4, c.label, "", 1, "L", false, 0, "")

		pdf.SetXY(x+3, y+7.5)
		pdf.SetFont("Helvetica", "B", 14)
		pdf.SetTextColor(15, 23, 42)
		pdf.CellFormat(cardW-6, 7, c.value, "", 1, "L", false, 0, "")

		pdf.SetXY(x+3, y+14.5)
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(cardW-6, 4, c.sub, "", 1, "L", false, 0, "")

		// Colored pill
		pdf.SetFillColor(c.r, c.g, c.b)
		pdf.RoundedRect(x+3, y+19.5, 16, 4.5, 1.5, "1234", "F")
		pdf.SetXY(x+3, y+19.5)
		pdf.SetFont("Helvetica", "B", 6.5)
		pdf.SetTextColor(255, 255, 255)
		pdf.CellFormat(16, 4.5, c.badge, "", 0, "C", false, 0, "")
	}
}

func drawBreakdownAndInsight(pdf *fpdf.Fpdf, report *BragReport) {
	y := pdf.GetY()

	// Left Card: Task Type Distribution + Stacked Progress Bar
	pdf.SetFillColor(255, 255, 255)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(14, y, 108, 32, 2, "1234", "FD")

	pdf.SetXY(18, y+3)
	pdf.SetFont("Helvetica", "B", 9.5)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(100, 5, "Work Distribution by Type", "", 1, "L", false, 0, "")

	// Horizontal stacked bar
	barX, barY, barW, barH := 18.0, y+11, 100.0, 6.0
	total := report.Summary.TotalTasks
	if total == 0 {
		pdf.SetFillColor(226, 232, 240)
		pdf.Rect(barX, barY, barW, barH, "F")
	} else {
		segments := []struct {
			key     string
			r, g, b int
		}{
			{"FEATURE", 19, 34, 71},
			{"BUG", 225, 29, 72},
			{"TASK", 14, 165, 233},
			{"SUBTASK", 16, 185, 129},
		}
		curX := barX
		for _, seg := range segments {
			cnt := report.TypeBreakdown[seg.key]
			if cnt == 0 {
				continue
			}
			segW := barW * float64(cnt) / float64(total)
			pdf.SetFillColor(seg.r, seg.g, seg.b)
			pdf.Rect(curX, barY, segW, barH, "F")
			curX += segW
		}
	}

	// Legend
	pdf.SetXY(18, y+21)
	pdf.SetFont("Helvetica", "", 7.5)
	pdf.SetTextColor(51, 65, 85)
	legend := fmt.Sprintf(
		"Features: %d   |   Bugs: %d   |   Tasks: %d   |   Subtasks: %d",
		report.TypeBreakdown["FEATURE"],
		report.TypeBreakdown["BUG"],
		report.TypeBreakdown["TASK"],
		report.TypeBreakdown["SUBTASK"],
	)
	pdf.CellFormat(100, 5, legend, "", 0, "L", false, 0, "")

	// Right Card: Key Insight Box
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(126, y, 70, 32, 2, "1234", "FD")

	pdf.SetXY(130, y+3)
	pdf.SetFont("Helvetica", "B", 9.5)
	pdf.SetTextColor(19, 34, 71)
	pdf.CellFormat(62, 5, "Key Insight", "", 1, "L", false, 0, "")

	pdf.SetXY(130, y+9.5)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(51, 65, 85)
	pdf.MultiCell(62, 4.2, safePDFText(report.KeyInsight), "", "L", false)
}

func drawProjectBreakdownTable(pdf *fpdf.Fpdf, report *BragReport) {
	pdf.SetFont("Helvetica", "B", 10.5)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(182, 6, "Impact by Project", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	// Table Header
	pdf.SetFillColor(241, 245, 249)
	pdf.SetDrawColor(226, 232, 240)
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(78, 7, "PROJECT", "1", 0, "L", true, 0, "")
	pdf.CellFormat(24, 7, "PREFIX", "1", 0, "C", true, 0, "")
	pdf.CellFormat(26, 7, "TOTAL", "1", 0, "C", true, 0, "")
	pdf.CellFormat(26, 7, "COMPLETED", "1", 0, "C", true, 0, "")
	pdf.CellFormat(28, 7, "COMPLETION", "1", 1, "C", true, 0, "")

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(15, 23, 42)
	if len(report.ProjectBreakdown) == 0 {
		pdf.CellFormat(182, 7, "No project activity recorded in this period.", "1", 1, "C", false, 0, "")
		return
	}

	for _, p := range report.ProjectBreakdown {
		pdf.CellFormat(78, 7, safePDFText(truncateStr(p.ProjectName, 40)), "1", 0, "L", false, 0, "")
		pdf.CellFormat(24, 7, safePDFText(p.Prefix), "1", 0, "C", false, 0, "")
		pdf.CellFormat(26, 7, fmt.Sprintf("%d", p.Total), "1", 0, "C", false, 0, "")
		pdf.CellFormat(26, 7, fmt.Sprintf("%d", p.Completed), "1", 0, "C", false, 0, "")
		pdf.CellFormat(28, 7, fmt.Sprintf("%.1f%%", p.CompletionRate), "1", 1, "C", false, 0, "")
	}
}

func drawWorkTable(pdf *fpdf.Fpdf, title string, items []BragTaskItem, completed bool) {
	if pdf.GetY() > 245 {
		pdf.AddPage()
	}

	pdf.SetFont("Helvetica", "B", 10.5)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(182, 6, fmt.Sprintf("%s (%d)", title, len(items)), "", 1, "L", false, 0, "")
	pdf.Ln(1)

	if len(items) == 0 {
		pdf.SetFont("Helvetica", "I", 8.5)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(182, 7, "No items in this category for the selected period.", "1", 1, "C", false, 0, "")
		return
	}

	for _, item := range items {
		if pdf.GetY() > 250 {
			pdf.AddPage()
		}

		// Item card
		summary := item.WorkSummary
		if strings.TrimSpace(summary) == "" {
			summary = item.Description
		}
		summary = safePDFText(truncateStr(summary, 320))

		progStr := "-"
		if item.ProgressPercent != nil {
			progStr = fmt.Sprintf("%d%%", *item.ProgressPercent)
		} else if completed {
			progStr = "100%"
		}

		y := pdf.GetY()
		pdf.SetFillColor(248, 250, 252)
		pdf.SetDrawColor(226, 232, 240)
		pdf.Rect(14, y, 182, 7, "FD")

		pdf.SetXY(16, y+1)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(19, 34, 71)
		pdf.CellFormat(22, 5, safePDFText(item.Key), "", 0, "L", false, 0, "")

		pdf.SetFont("Helvetica", "B", 7)
		pdf.SetTextColor(225, 29, 72)
		pdf.CellFormat(20, 5, safePDFText(item.TicketType), "", 0, "L", false, 0, "")

		pdf.SetFont("Helvetica", "B", 8.5)
		pdf.SetTextColor(15, 23, 42)
		pdf.CellFormat(98, 5, safePDFText(truncateStr(item.Title, 58)), "", 0, "L", false, 0, "")

		pdf.SetFont("Helvetica", "", 7.5)
		pdf.SetTextColor(71, 85, 105)
		pdf.CellFormat(25, 5, safePDFText(item.Status), "", 0, "R", false, 0, "")
		pdf.CellFormat(13, 5, progStr, "", 1, "R", false, 0, "")

		// Work Summary Body
		pdf.SetXY(16, pdf.GetY()+1)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(51, 65, 85)
		pdf.MultiCell(178, 4.2, summary, "", "L", false)
		pdf.Ln(2)
	}
}

func firstWord(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "Engineer"
	}
	return parts[0]
}

func truncateStr(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func safePDFText(s string) string {
	replacer := strings.NewReplacer(
		"—", "-",
		"–", "-",
		"“", "\"",
		"”", "\"",
		"’", "'",
		"‘", "'",
		"•", "*",
	)
	s = replacer.Replace(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 255 {
			b.WriteRune(r)
		} else if r == '\n' || r == '\t' {
			b.WriteRune(' ')
		}
	}
	return b.String()
}

