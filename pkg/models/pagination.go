package models

// PaginationInfo représente les informations de pagination
// @Description Informations de pagination pour les listes
type PaginationInfo struct {
	Page         int  `json:"page" example:"1"`
	PageSize     int  `json:"page_size" example:"25"`
	TotalPages   int  `json:"total_pages" example:"4"`
	TotalItems   int  `json:"total_items" example:"95"`
	HasNext      bool `json:"has_next" example:"true"`
	HasPrevious  bool `json:"has_previous" example:"false"`
	NextPage     int  `json:"next_page,omitempty" example:"2"`
	PreviousPage int  `json:"previous_page,omitempty"`
} // @name PaginationInfo
