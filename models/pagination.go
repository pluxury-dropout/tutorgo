package models

type Pagination struct {
	Page  int `form:"page"`
	Limit int `form:"limit"`
}

func (p *Pagination) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 || p.Limit > 100 {
		p.Limit = 20
	}
}

func (p Pagination) Offset() int {
	return (p.Page - 1) * p.Limit
}

type PagedResponse[T any] struct {
	Data  []T `json:"data"`
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}
