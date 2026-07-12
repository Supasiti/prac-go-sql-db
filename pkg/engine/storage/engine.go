package storage

import "github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"

type StorageEngine interface {
	ReadPage(id types.PageID) (*types.Page, error)
	WritePage(id types.PageID, page *types.Page) error
	AllocatePage() (types.PageID, error)
	Sync() error
	Close() error
}
