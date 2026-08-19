package domain

import "regexp"

// mentionSub matches @sub tokens used in chat (workspace member subs, not display names).
var mentionSub = regexp.MustCompile(`@([A-Za-z0-9._-]+)`)

func ExtractMentions(body string, memberSubs []string) []string {
	allow := make(map[string]bool, len(memberSubs))
	for _, sub := range memberSubs {
		if sub != "" {
			allow[sub] = true
		}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, m := range mentionSub.FindAllStringSubmatch(body, -1) {
		sub := m[1]
		if !allow[sub] || seen[sub] {
			continue
		}
		seen[sub] = true
		out = append(out, sub)
	}
	return out
}

func ValidFilePurpose(purpose string) bool {
	return purpose == PurposeWiki || purpose == PurposeChat
}
