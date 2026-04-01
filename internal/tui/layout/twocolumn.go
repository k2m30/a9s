package layout

import "strings"

// TwoColumn renders two framed panels side by side within totalW columns.
// leftLines and rightLines are pre-rendered content lines (what goes inside the frame).
// leftTitle and rightTitle are centered in each panel's top border.
// totalW is the total available width. leftW is the width of the left panel (including borders).
// Right panel width = totalW - leftW. h is total height for both panels (including borders).
func TwoColumn(leftLines, rightLines []string, leftTitle, rightTitle string, totalW, leftW, h int) string {
	rightW := totalW - leftW

	leftFrame := RenderFrame(leftLines, leftTitle, leftW, h)
	rightFrame := RenderFrame(rightLines, rightTitle, rightW, h)

	leftRows := strings.Split(leftFrame, "\n")
	rightRows := strings.Split(rightFrame, "\n")

	var sb strings.Builder
	for i := 0; i < len(leftRows) || i < len(rightRows); i++ {
		if i > 0 {
			sb.WriteString("\n")
		}
		left := ""
		if i < len(leftRows) {
			left = leftRows[i]
		}
		right := ""
		if i < len(rightRows) {
			right = rightRows[i]
		}
		sb.WriteString(left)
		sb.WriteString(right)
	}
	return sb.String()
}
