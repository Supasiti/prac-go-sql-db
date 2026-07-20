package buffer

import (
	"container/list"
	"errors"
	"fmt"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/storage"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

var ErrNoEvictablePage = errors.New("no evictable page in pool")

type Pool struct {
	engine  storage.StorageEngine
	frames  map[types.PageID]*Frame
	lru     *list.List
	maxSize int
}

type Frame struct {
	page    *types.Page
	pinCount int
	dirty    bool
	element  *list.Element
}

func NewPool(engine storage.StorageEngine, maxSize int) *Pool {
	return &Pool{
		engine:  engine,
		frames:  make(map[types.PageID]*Frame),
		lru:     list.New(),
		maxSize: maxSize,
	}
}

func (p *Pool) FetchPage(id types.PageID) (*types.Page, error) {
	if frame, ok := p.frames[id]; ok {
		p.lru.MoveToFront(frame.element)
		frame.pinCount++
		return frame.page, nil
	}

	if p.lru.Len() >= p.maxSize {
		if _, err := p.Evict(); err != nil {
			return nil, fmt.Errorf("evict: %w", err)
		}
	}

	page, err := p.engine.ReadPage(id)
	if err != nil {
		return nil, fmt.Errorf("read page: %w", err)
	}

	frame := &Frame{
		page:     page,
		pinCount: 1,
	}
	frame.element = p.lru.PushFront(frame)
	p.frames[id] = frame

	return page, nil
}

func (p *Pool) ReleasePage(id types.PageID) {
	frame, ok := p.frames[id]
	if !ok {
		return
	}
	if frame.pinCount > 0 {
		frame.pinCount--
	}
}

func (p *Pool) MarkDirty(id types.PageID) {
	if frame, ok := p.frames[id]; ok {
		frame.dirty = true
	}
}

func (p *Pool) FlushPage(id types.PageID) error {
	frame, ok := p.frames[id]
	if !ok || !frame.dirty {
		return nil
	}

	if err := p.engine.WritePage(frame.page); err != nil {
		return fmt.Errorf("write page: %w", err)
	}

	frame.dirty = false
	return nil
}

func (p *Pool) Flush() error {
	for id := range p.frames {
		if err := p.FlushPage(id); err != nil {
			return err
		}
	}
	return nil
}

func (p *Pool) Evict() (types.PageID, error) {
	for e := p.lru.Back(); e != nil; e = e.Prev() {
		frame := e.Value.(*Frame)
		if frame.pinCount > 0 {
			continue
		}

		if frame.dirty {
			if err := p.engine.WritePage(frame.page); err != nil {
				return 0, fmt.Errorf("flush during eviction: %w", err)
			}
		}

		id := frame.page.ID
		p.lru.Remove(e)
		delete(p.frames, id)
		return id, nil
	}

	return 0, ErrNoEvictablePage
}

func (p *Pool) PinCount(id types.PageID) int {
	if frame, ok := p.frames[id]; ok {
		return frame.pinCount
	}
	return 0
}

func (p *Pool) Sync() error {
	return p.engine.Sync()
}
