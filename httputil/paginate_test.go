package httputil

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPaginated(t *testing.T) {
	t.Parallel()
	p := NewPaginated([]string{"a", "b"}, 10, 1, 2)
	require.NotNil(t, p)
	assert.Equal(t, []string{"a", "b"}, p.Data)
	assert.Equal(t, int64(10), p.Total)
	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 2, p.PerPage)
	assert.Equal(t, 5, p.TotalPages)

	p2 := NewPaginated([]int{}, 0, 1, 10)
	assert.Equal(t, 0, p2.TotalPages)
}

func TestFetchPage_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fetchFn := func(_ context.Context, _, offset int) ([]int, error) {
		return []int{offset + 1, offset + 2}, nil
	}
	countFn := func(_ context.Context) (int64, error) {
		return 100, nil
	}
	p, err := FetchPage(ctx, 2, 10, fetchFn, countFn)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, []int{11, 12}, p.Data)
	assert.Equal(t, int64(100), p.Total)
	assert.Equal(t, 2, p.Page)
	assert.Equal(t, 10, p.PerPage)
	assert.Equal(t, 10, p.TotalPages)
}

func TestFetchPage_FetchError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fetchErr := errors.New("fetch failed")
	fetchFn := func(_ context.Context, _, _ int) ([]int, error) {
		return nil, fetchErr
	}
	countFn := func(_ context.Context) (int64, error) {
		return 0, nil
	}
	_, err := FetchPage(ctx, 1, 10, fetchFn, countFn)
	require.Error(t, err)
	assert.ErrorIs(t, err, fetchErr)
}

func TestFetchPage_CountError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	countErr := errors.New("count failed")
	fetchFn := func(_ context.Context, _, _ int) ([]int, error) {
		return nil, nil
	}
	countFn := func(_ context.Context) (int64, error) {
		return 0, countErr
	}
	_, err := FetchPage(ctx, 1, 10, fetchFn, countFn)
	require.Error(t, err)
	assert.ErrorIs(t, err, countErr)
}
