package service

import (
	"strings"
	"time"

	"github.com/portfolio/pf-workspace/api/internal/domain"
)

func (s *Service) CreateSprint(sub, boardID, name string, startAt, endAt time.Time) (domain.Sprint, error) {
	wsID, err := s.store.BoardWorkspaceID(boardID)
	if err != nil {
		return domain.Sprint{}, err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Sprint{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > domain.MaxSprintName {
		return domain.Sprint{}, domain.ErrInvalid
	}
	startAt = startAt.UTC()
	endAt = endAt.UTC()
	if !domain.ValidSprintWindow(startAt, endAt) {
		return domain.Sprint{}, domain.ErrInvalid
	}
	return s.store.CreateSprint(boardID, name, startAt, endAt, s.now().UTC())
}

func (s *Service) ListSprints(sub, boardID string) ([]domain.Sprint, error) {
	wsID, err := s.store.BoardWorkspaceID(boardID)
	if err != nil {
		return nil, err
	}
	if err := s.requireRead(wsID, sub); err != nil {
		return nil, err
	}
	return s.store.ListSprints(boardID)
}

func (s *Service) GetSprint(sub, sprintID string) (domain.Sprint, error) {
	sp, err := s.store.GetSprint(sprintID)
	if err != nil {
		return domain.Sprint{}, err
	}
	if err := s.requireRead(sp.WorkspaceID, sub); err != nil {
		return domain.Sprint{}, err
	}
	return sp, nil
}

func (s *Service) UpdateSprint(sub, sprintID, name string, startAt, endAt *time.Time) (domain.Sprint, error) {
	sp, err := s.store.GetSprint(sprintID)
	if err != nil {
		return domain.Sprint{}, err
	}
	if err := s.requireWrite(sp.WorkspaceID, sub); err != nil {
		return domain.Sprint{}, err
	}
	name = strings.TrimSpace(name)
	if name != "" && len(name) > domain.MaxSprintName {
		return domain.Sprint{}, domain.ErrInvalid
	}
	nextStart := sp.StartAt
	nextEnd := sp.EndAt
	if startAt != nil {
		nextStart = startAt.UTC()
	}
	if endAt != nil {
		nextEnd = endAt.UTC()
	}
	if !domain.ValidSprintWindow(nextStart, nextEnd) {
		return domain.Sprint{}, domain.ErrInvalid
	}
	var startPtr, endPtr *time.Time
	if startAt != nil {
		t := nextStart
		startPtr = &t
	}
	if endAt != nil {
		t := nextEnd
		endPtr = &t
	}
	return s.store.UpdateSprint(sprintID, name, startPtr, endPtr, s.now().UTC())
}

func (s *Service) DeleteSprint(sub, sprintID string) error {
	sp, err := s.store.GetSprint(sprintID)
	if err != nil {
		return err
	}
	if err := s.requireWrite(sp.WorkspaceID, sub); err != nil {
		return err
	}
	return s.store.DeleteSprint(sprintID)
}

func (s *Service) SprintBurndown(sub, sprintID string) (domain.Burndown, error) {
	sp, err := s.GetSprint(sub, sprintID)
	if err != nil {
		return domain.Burndown{}, err
	}
	cards, err := s.store.ListCardsOnBoard(sp.BoardID)
	if err != nil {
		return domain.Burndown{}, err
	}
	return domain.BurndownFor(sp, cards), nil
}

func (s *Service) ListPageVersions(sub, pageID string) ([]domain.PageVersionInfo, error) {
	if _, err := s.GetPage(sub, pageID); err != nil {
		return nil, err
	}
	vers, err := s.store.ListPageVersions(pageID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PageVersionInfo, 0, len(vers))
	for _, v := range vers {
		out = append(out, v.Info())
	}
	return out, nil
}

func (s *Service) GetPageVersion(sub, pageID string, number int) (domain.PageVersion, error) {
	if _, err := s.GetPage(sub, pageID); err != nil {
		return domain.PageVersion{}, err
	}
	return s.store.GetPageVersion(pageID, number)
}

func (s *Service) DiffPageVersions(sub, pageID string, from, to int) (domain.PageDiff, error) {
	if _, err := s.GetPage(sub, pageID); err != nil {
		return domain.PageDiff{}, err
	}
	if from < 1 || to < 1 || from == to {
		return domain.PageDiff{}, domain.ErrInvalid
	}
	a, err := s.store.GetPageVersion(pageID, from)
	if err != nil {
		return domain.PageDiff{}, err
	}
	b, err := s.store.GetPageVersion(pageID, to)
	if err != nil {
		return domain.PageDiff{}, err
	}
	return domain.DiffPages(pageID, a, b), nil
}

func (s *Service) RestorePageVersion(sub, pageID string, number, lockVersion int) (domain.Page, error) {
	wsID, err := s.store.PageWorkspaceID(pageID)
	if err != nil {
		return domain.Page{}, err
	}
	if err := s.requireWrite(wsID, sub); err != nil {
		return domain.Page{}, err
	}
	snap, err := s.store.GetPageVersion(pageID, number)
	if err != nil {
		return domain.Page{}, err
	}
	title := snap.Title
	body := snap.Body
	return s.UpdatePage(sub, pageID, &title, &body, nil, nil, lockVersion)
}
