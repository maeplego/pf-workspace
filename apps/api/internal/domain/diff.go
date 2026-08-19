package domain

import "strings"

func SplitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

// LineDiff is a line-level LCS diff (equal / delete / insert). Plain text only; not a Y.Doc diff.
func LineDiff(from, to string) []DiffLine {
	a := SplitLines(from)
	b := SplitLines(to)
	if len(a) == 0 && len(b) == 0 {
		return []DiffLine{}
	}
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var out []DiffLine
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, DiffLine{Op: "equal", Text: a[i]})
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			out = append(out, DiffLine{Op: "delete", Text: a[i]})
			i++
		} else {
			out = append(out, DiffLine{Op: "insert", Text: b[j]})
			j++
		}
	}
	for i < n {
		out = append(out, DiffLine{Op: "delete", Text: a[i]})
		i++
	}
	for j < m {
		out = append(out, DiffLine{Op: "insert", Text: b[j]})
		j++
	}
	return out
}

func DiffPages(pageID string, fromVer, toVer PageVersion) PageDiff {
	return PageDiff{
		PageID:       pageID,
		From:         fromVer.Number,
		To:           toVer.Number,
		TitleChanged: fromVer.Title != toVer.Title,
		FromTitle:    fromVer.Title,
		ToTitle:      toVer.Title,
		Lines:        LineDiff(fromVer.Body, toVer.Body),
	}
}
