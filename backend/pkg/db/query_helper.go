package db

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// FilterEqual filters records by exact match of field values
// It adds WHERE conditions for each specified field if the query parameter exists
func FilterEqual(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok {
			db = db.Where(f+" = ?", q)
		}
	}
	return db
}

// FilterLike filters records by partial match (LIKE) of field values
// It adds WHERE conditions with OR for each specified field if the query parameter exists
func FilterLike(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	likes := false
	d := DB
	for _, f := range fields {
		if q, ok := ctx.GetQuery(f); ok && q != "" {
			d = d.Or(f+" LIKE ?", "%"+q+"%")
			likes = true
		}
	}
	if !likes {
		return db
	}
	return db.Where(d)
}

// FilterSearch performs a search across multiple fields using a single search parameter
// It looks for the "search" query parameter and searches all specified fields
func FilterSearch(ctx *gin.Context, db *gorm.DB, fields ...string) *gorm.DB {
	q, ok := ctx.GetQuery("search")
	if !ok || len(fields) <= 0 {
		return db
	}

	d := DB
	for _, f := range fields {
		d = d.Or(f+" LIKE ?", "%"+q+"%")
	}

	return db.Where(d)
}
