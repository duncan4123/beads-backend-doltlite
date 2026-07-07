//go:build cgo

package doltlite

import (
	"context"
	"database/sql"

	"github.com/duncan4123/beads-backend-doltlite/internal/storage/issueops"
	"github.com/duncan4123/beads-backend-doltlite/internal/types"
)

func (s *DoltliteStore) SearchIssues(ctx context.Context, query string, filter types.IssueFilter) ([]*types.Issue, error) {
	var result []*types.Issue
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.SearchIssuesSQLiteInTx(ctx, tx, query, filter)
		return err
	})
	return result, err
}

func (s *DoltliteStore) SearchIssueIDs(ctx context.Context, query string, filter types.IssueFilter) ([]string, error) {
	issues, err := s.SearchIssues(ctx, query, filter)
	if err != nil {
		return nil, err
	}
	return issueIDsFromIssues(issues), nil
}

func (s *DoltliteStore) SearchIssuesWithCounts(ctx context.Context, query string, filter types.IssueFilter) ([]*types.IssueWithCounts, error) {
	var result []*types.IssueWithCounts
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.SearchIssuesWithCountsSQLiteInTx(ctx, tx, query, filter)
		return err
	})
	return result, err
}

func issueIDsFromIssues(issues []*types.Issue) []string {
	ids := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue != nil {
			ids = append(ids, issue.ID)
		}
	}
	return ids
}

func (s *DoltliteStore) ListWisps(ctx context.Context, filter types.WispFilter) ([]*types.Issue, error) {
	issueFilter := issueops.WispFilterToIssueFilter(filter)
	var result []*types.Issue
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.SearchIssuesSQLiteInTx(ctx, tx, "", issueFilter)
		return err
	})
	return result, err
}

func (s *DoltliteStore) GetLabelsForIssues(ctx context.Context, issueIDs []string) (map[string][]string, error) {
	var result map[string][]string
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.GetLabelsForIssuesInTx(ctx, tx, issueIDs)
		return err
	})
	return result, err
}

func (s *DoltliteStore) GetCommentCounts(ctx context.Context, issueIDs []string) (map[string]int, error) {
	var result map[string]int
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.GetCommentCountsInTx(ctx, tx, issueIDs)
		return err
	})
	return result, err
}

func (s *DoltliteStore) GetAllDependencyRecords(ctx context.Context) (map[string][]*types.Dependency, error) {
	var result map[string][]*types.Dependency
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.GetAllDependencyRecordsInTx(ctx, tx)
		return err
	})
	return result, err
}

func (s *DoltliteStore) GetDependencyRecordsForIssues(ctx context.Context, issueIDs []string) (map[string][]*types.Dependency, error) {
	var result map[string][]*types.Dependency
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.GetDependencyRecordsForIssuesInTx(ctx, tx, issueIDs)
		return err
	})
	return result, err
}

func (s *DoltliteStore) GetDependencyCounts(ctx context.Context, issueIDs []string) (map[string]*types.DependencyCounts, error) {
	var result map[string]*types.DependencyCounts
	err := s.withConn(ctx, false, func(tx *sql.Tx) error {
		var err error
		result, err = issueops.GetDependencyCountsInTx(ctx, tx, issueIDs)
		return err
	})
	return result, err
}

func (s *DoltliteStore) GetBlockingInfoForIssues(ctx context.Context, issueIDs []string) (
	blockedByMap map[string][]string,
	blocksMap map[string][]string,
	parentMap map[string]string,
	err error,
) {
	err = s.withConn(ctx, false, func(tx *sql.Tx) error {
		var txErr error
		blockedByMap, blocksMap, parentMap, txErr = issueops.GetBlockingInfoForIssuesInTx(ctx, tx, issueIDs)
		return txErr
	})
	return
}
