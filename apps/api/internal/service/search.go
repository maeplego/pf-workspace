package service

import (
	"strconv"
	"strings"

	"github.com/portfolio/pf-workspace/api/internal/domain"
)

func (s *Service) Search(sub, wsID, q, typesRaw string) ([]domain.SearchHit, error) {
	if err := s.requireRead(wsID, sub); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, domain.ErrInvalid
	}
	types, err := domain.ParseSearchTypes(typesRaw)
	if err != nil {
		return nil, err
	}
	role, err := s.store.MemberRole(wsID, sub)
	if err != nil {
		return nil, err
	}
	hits := []domain.SearchHit{}
	for _, t := range types {
		switch t {
		case "page":
			part, err := s.searchPages(wsID, role, q)
			if err != nil {
				return nil, err
			}
			hits = append(hits, part...)
		case "document":
			part, err := s.searchDocuments(wsID, q)
			if err != nil {
				return nil, err
			}
			hits = append(hits, part...)
		case "board":
			part, err := s.searchBoards(wsID, q)
			if err != nil {
				return nil, err
			}
			hits = append(hits, part...)
		case "card":
			part, err := s.searchCards(wsID, q)
			if err != nil {
				return nil, err
			}
			hits = append(hits, part...)
		case "channel":
			part, err := s.searchChannels(wsID, q)
			if err != nil {
				return nil, err
			}
			hits = append(hits, part...)
		case "message":
			part, err := s.searchMessages(wsID, q)
			if err != nil {
				return nil, err
			}
			hits = append(hits, part...)
		}
	}
	return hits, nil
}

func (s *Service) searchPages(wsID string, role domain.Role, q string) ([]domain.SearchHit, error) {
	pages, err := s.store.ListPages(wsID)
	if err != nil {
		return nil, err
	}
	if role == domain.RoleGuest {
		pages = domain.FilterGuestPages(pages)
	}
	var hits []domain.SearchHit
	for _, p := range pages {
		if p.ArchivedAt != nil {
			continue
		}
		titleMatch := domain.ContainsFold(p.Title, q)
		bodyMatch := domain.ContainsFold(p.Body, q)
		if !titleMatch && !bodyMatch {
			continue
		}
		matchLabel := "本文"
		snippet := domain.Snippet(p.Body, q, 80)
		if titleMatch {
			matchLabel = "ページ名"
			snippet = p.Title
		}
		hits = append(hits, domain.SearchHit{
			Type:       "page",
			ID:         p.ID,
			Title:      p.Title,
			Context:    "Wiki · " + p.Status,
			MatchLabel: matchLabel,
			Snippet:    snippet,
			HrefHints: map[string]string{
				"workspaceId": wsID,
				"pageId":      p.ID,
			},
		})
	}
	return hits, nil
}

func (s *Service) searchDocuments(wsID, q string) ([]domain.SearchHit, error) {
	docs, err := s.store.ListDocuments(wsID)
	if err != nil {
		return nil, err
	}
	var hits []domain.SearchHit
	for _, d := range docs {
		if d.DeletedAt != nil {
			continue
		}
		titleMatch := domain.ContainsFold(d.Title, q)
		bodyMatch := domain.ContainsFold(d.Body, q)
		if !titleMatch && !bodyMatch {
			continue
		}
		matchLabel := "本文"
		snippet := domain.Snippet(d.Body, q, 80)
		if titleMatch {
			matchLabel = "ドキュメント名"
			snippet = d.Title
		}
		hits = append(hits, domain.SearchHit{
			Type:       "document",
			ID:         d.ID,
			Title:      d.Title,
			Context:    "Docs",
			MatchLabel: matchLabel,
			Snippet:    snippet,
			HrefHints: map[string]string{
				"workspaceId": wsID,
				"documentId":  d.ID,
			},
		})
	}
	return hits, nil
}

func (s *Service) searchBoards(wsID, q string) ([]domain.SearchHit, error) {
	boards, err := s.store.ListBoards(wsID)
	if err != nil {
		return nil, err
	}
	var hits []domain.SearchHit
	for _, b := range boards {
		if b.ArchivedAt != nil || !domain.ContainsFold(b.Name, q) {
			continue
		}
		hits = append(hits, domain.SearchHit{
			Type:       "board",
			ID:         b.ID,
			Title:      b.Name,
			MatchLabel: "ボード名",
			Snippet:    b.Name,
			HrefHints: map[string]string{
				"boardId": b.ID,
			},
		})
	}
	return hits, nil
}

func (s *Service) searchCards(wsID, q string) ([]domain.SearchHit, error) {
	cards, err := s.store.ListCardsInWorkspace(wsID)
	if err != nil {
		return nil, err
	}
	boards, err := s.store.ListBoards(wsID)
	if err != nil {
		return nil, err
	}
	boardName := map[string]string{}
	archived := map[string]bool{}
	for _, b := range boards {
		boardName[b.ID] = b.Name
		archived[b.ID] = b.ArchivedAt != nil
	}
	var hits []domain.SearchHit
	for _, c := range cards {
		if archived[c.BoardID] {
			continue
		}
		blob := c.Title + "\n" + c.Description
		if !domain.ContainsFold(blob, q) {
			continue
		}
		bname := boardName[c.BoardID]
		matchLabel := "カード名"
		snippet := c.Title
		if !domain.ContainsFold(c.Title, q) {
			matchLabel = "説明"
			snippet = domain.Snippet(c.Description, q, 80)
		}
		hits = append(hits, domain.SearchHit{
			Type:       "card",
			ID:         c.ID,
			Title:      c.Title,
			Context:    "ボード · " + bname,
			MatchLabel: matchLabel,
			Snippet:    snippet,
			HrefHints: map[string]string{
				"boardId": c.BoardID,
				"cardId":  c.ID,
			},
		})
	}
	return hits, nil
}

func (s *Service) searchChannels(wsID, q string) ([]domain.SearchHit, error) {
	channels, err := s.store.ListChannels(wsID)
	if err != nil {
		return nil, err
	}
	var hits []domain.SearchHit
	for _, ch := range channels {
		if !domain.ContainsFold(ch.Name, q) {
			continue
		}
		hits = append(hits, domain.SearchHit{
			Type:       "channel",
			ID:         ch.ID,
			Title:      ch.Name,
			MatchLabel: "チャンネル名",
			Snippet:    ch.Name,
			HrefHints: map[string]string{
				"channelId": ch.ID,
			},
		})
	}
	return hits, nil
}

func (s *Service) searchMessages(wsID, q string) ([]domain.SearchHit, error) {
	channels, err := s.store.ListChannels(wsID)
	if err != nil {
		return nil, err
	}
	var hits []domain.SearchHit
	for _, ch := range channels {
		msgs, err := s.store.ListMessages(ch.ID, 0)
		if err != nil {
			return nil, err
		}
		for _, m := range msgs {
			if !domain.ContainsFold(m.Body, q) {
				continue
			}
			hits = append(hits, domain.SearchHit{
				Type:       "message",
				ID:         m.ID,
				Title:      m.Body,
				Context:    "チャンネル · " + ch.Name,
				MatchLabel: "メッセージ",
				Snippet:    domain.Snippet(m.Body, q, 80),
				HrefHints: map[string]string{
					"channelId": ch.ID,
					"seq":       strconv.Itoa(m.Seq),
				},
			})
		}
	}
	return hits, nil
}
