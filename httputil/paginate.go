package httputil

import (
	"context"
	"errors"
	"math"

	"golang.org/x/sync/errgroup"
)

// Paginated holds a page of items and pagination metadata for JSON responses.
type Paginated[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PerPage    int   `json:"per_page"`
	TotalPages int   `json:"total_pages"`
}

// NewPaginated builds a Paginated with TotalPages computed from total and perPage.
func NewPaginated[T any](data []T, total int64, page, perPage int) *Paginated[T] {
	return &Paginated[T]{
		Data:       data,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: TotalPages(total, perPage),
	}
}

// FetchPage runs fetchFn and countFn in parallel and returns a Paginated result. Page and perPage are clamped to at least 1.
func FetchPage[T any](
	ctx context.Context,
	page, perPage int,
	fetchFn func(ctx context.Context, limit, offset int) ([]T, error),
	countFn func(ctx context.Context) (int64, error),
) (*Paginated[T], error) {
	if fetchFn == nil || countFn == nil {
		return nil, errors.New("fetchFn and countFn must be non-nil")
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 1
	}
	offset64 := int64(page-1) * int64(perPage)
	if offset64 < 0 || offset64 > math.MaxInt {
		offset64 = math.MaxInt
	}
	offset := int(offset64)
	var items []T
	var total int64

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		items, err = fetchFn(gCtx, perPage, offset)
		return err
	})
	g.Go(func() error {
		var err error
		total, err = countFn(gCtx)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return NewPaginated(items, total, page, perPage), nil
}
