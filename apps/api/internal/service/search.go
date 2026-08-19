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
		case "card":
			part, err := s.searchCards(wsID, q)
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
		blob := p.Title + "\n" + p.Body
		if !domain.ContainsFold(blob, q) {
			continue
		}
		hits = append(hits, domain.SearchHit{
			Type:    "page",
			ID:      p.ID,
			Title:   p.Title,
			Snippet: domain.Snippet(blob, q, 80),
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
		blob := d.Title + "\n" + d.Body
		if !domain.ContainsFold(blob, q) {
			continue
		}
		hits = append(hits, domain.SearchHit{
			Type:    "document",
			ID:      d.ID,
			Title:   d.Title,
			Snippet: domain.Snippet(blob, q, 80),
			HrefHints: map[string]string{
				"workspaceId": wsID,
				"documentId":  d.ID,
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
	var hits []domain.SearchHit
	for _, c := range cards {
		blob := c.Title + "\n" + c.Description
		if !domain.ContainsFold(blob, q) {
			continue
		}
		hits = append(hits, domain.SearchHit{
			Type:    "card",
			ID:      c.ID,
			Title:   c.Title,
			Snippet: domain.Snippet(blob, q, 80),
			HrefHints: map[string]string{
				"boardId": c.BoardID,
				"cardId":  c.ID,
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
			title := ch.Name
			hits = append(hits, domain.SearchHit{
				Type:    "message",
				ID:      m.ID,
				Title:   title,
				Snippet: domain.Snippet(m.Body, q, 80),
				HrefHints: map[string]string{
					"channelId": ch.ID,
					"seq":       strconv.Itoa(m.Seq),
				},
			})
		}
	}
	return hits, nil
}
