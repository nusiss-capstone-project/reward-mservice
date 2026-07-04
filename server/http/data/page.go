package data

const (
	DefaultPage = 1
	DefaultSize = 20
	MaxSize     = 100
)

type PageQuery struct {
	Page int `form:"page"`
	Size int `form:"size"`
}

func (p PageQuery) Normalize() (page, size int) {
	page = p.Page
	size = p.Size
	if page <= 0 {
		page = DefaultPage
	}
	if size <= 0 {
		size = DefaultSize
	}
	if size > MaxSize {
		size = MaxSize
	}
	return page, size
}

type PageResult struct {
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
	Items interface{} `json:"items"`
}
