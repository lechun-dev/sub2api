package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryAggregateUsageStatsByUser(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	filters := usagestats.UsageLogFilters{UserID: 7, StartTime: &start, EndTime: &end}

	mock.ExpectQuery(`SELECT COUNT\(\*\),`).
		WithArgs(int64(7), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_cost"}).
			AddRow(2, 100, 200, 10, 20, 1.5))

	mock.ExpectQuery(`SELECT ul.user_id AS group_user_id`).
		WithArgs(int64(7), start, end, 50).
		WillReturnRows(sqlmock.NewRows([]string{"group_user_id", "group_username", "group_email", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_cost"}).
			AddRow(int64(7), "张三", "zhang@example.com", 2, 100, 200, 10, 20, 1.5))

	items, totals, err := repo.AggregateUsageStats(context.Background(), filters, service.UsageStatsGroupUser, 50, true)
	require.NoError(t, err)
	require.Equal(t, int64(2), totals.Requests)
	require.Equal(t, int64(300), totals.TotalTokens)
	require.Equal(t, 1.5, totals.TotalCost)
	require.Len(t, items, 1)
	require.Equal(t, "7", items[0].Key)
	require.Equal(t, "张三", items[0].Username)
	require.Equal(t, int64(300), items[0].TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryAggregateUsageStatsTotalsOnlyWithUsername(t *testing.T) {
	t.Parallel()

	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	filters := usagestats.UsageLogFilters{UserName: "张三", StartTime: &start, EndTime: &end}

	mock.ExpectQuery(`LEFT JOIN users u ON u.id = ul.user_id WHERE \(u.username ILIKE`).
		WithArgs("%张三%", start, end).
		WillReturnRows(sqlmock.NewRows([]string{"requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_cost"}).
			AddRow(5, 10, 20, 0, 0, 0.5))

	items, totals, err := repo.AggregateUsageStats(context.Background(), filters, service.UsageStatsGroupNone, 50, true)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, int64(5), totals.Requests)
	require.Equal(t, int64(30), totals.TotalTokens)
	require.NoError(t, mock.ExpectationsWereMet())
}
