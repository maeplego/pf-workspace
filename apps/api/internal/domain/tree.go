package domain

import "sort"

func BuildPageTree(pages []Page) []PageNode {
	byParent := map[string][]Page{}
	for _, p := range pages {
		byParent[p.ParentID] = append(byParent[p.ParentID], p)
	}
	for parent, kids := range byParent {
		sort.Slice(kids, func(i, j int) bool {
			if kids[i].Position != kids[j].Position {
				return kids[i].Position < kids[j].Position
			}
			return kids[i].CreatedAt.Before(kids[j].CreatedAt)
		})
		byParent[parent] = kids
	}
	return buildNodes(byParent, "")
}

func buildNodes(byParent map[string][]Page, parentID string) []PageNode {
	kids := byParent[parentID]
	out := make([]PageNode, 0, len(kids))
	for _, p := range kids {
		out = append(out, PageNode{
			ID:       p.ID,
			ParentID: p.ParentID,
			Title:    p.Title,
			Status:   p.Status,
			Position: p.Position,
			Children: buildNodes(byParent, p.ID),
		})
	}
	return out
}

// FilterGuestPages drops drafts and published pages whose ancestor is not published.
func FilterGuestPages(pages []Page) []Page {
	byID := make(map[string]Page, len(pages))
	for _, p := range pages {
		byID[p.ID] = p
	}
	var out []Page
	for _, p := range pages {
		if !PageVisibleToGuest(p) {
			continue
		}
		ok := true
		cur := p.ParentID
		seen := map[string]bool{}
		for cur != "" {
			if seen[cur] {
				ok = false
				break
			}
			seen[cur] = true
			par, exists := byID[cur]
			if !exists || !PageVisibleToGuest(par) {
				ok = false
				break
			}
			cur = par.ParentID
		}
		if ok {
			out = append(out, p)
		}
	}
	return out
}
