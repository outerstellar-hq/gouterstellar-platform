package model

import "math"

type PaginationMetadata struct {
	CurrentPage  int
	PageSize     int
	TotalItems   int64
	TotalPages   int
	HasPrevious  bool
	HasNext      bool
	PreviousPage *int
	NextPage     *int
}

func NewPaginationMetadata(currentPage, pageSize int, totalItems int64) PaginationMetadata {
	totalPages := int(math.Ceil(float64(totalItems) / float64(pageSize)))
	if totalPages == 0 {
		totalPages = 1
	}
	md := PaginationMetadata{
		CurrentPage: currentPage,
		PageSize:    pageSize,
		TotalItems:  totalItems,
		TotalPages:  totalPages,
		HasPrevious: currentPage > 1,
		HasNext:     currentPage < totalPages,
	}
	if md.HasPrevious {
		p := currentPage - 1
		md.PreviousPage = &p
	}
	if md.HasNext {
		n := currentPage + 1
		md.NextPage = &n
	}
	return md
}

type PagedResult[T any] struct {
	Items    []T
	Metadata PaginationMetadata
}
