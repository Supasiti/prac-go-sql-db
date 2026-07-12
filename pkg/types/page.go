package types

type PageID uint64

const PageSize = 4096

type Page struct {
	ID   PageID
	Data [PageSize]byte
}
