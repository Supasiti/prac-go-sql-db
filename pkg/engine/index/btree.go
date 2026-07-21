package index

import (
	"bytes"
	"errors"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/engine/buffer"
	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

var (
	ErrKeyNotFound  = errors.New("btree: key not found")
	ErrDuplicateKey = errors.New("btree: duplicate key")
)

type BTree struct {
	pool    *buffer.Pool
	rootID  types.PageID
	keySize int
	order   int
}

func NewBTree(pool *buffer.Pool, rootID types.PageID, keySize, order int) *BTree {
	return &BTree{
		pool:    pool,
		rootID:  rootID,
		keySize: keySize,
		order:   order,
	}
}

func (t *BTree) readNode(pageID types.PageID) (*Node, error) {
	page, err := t.pool.FetchPage(pageID)
	if err != nil {
		return nil, err
	}
	node := Deserialize(page.Data[:], pageID)
	t.pool.ReleasePage(pageID)
	return node, nil
}

func (t *BTree) writeNode(node *Node) error {
	page, err := t.pool.FetchPage(node.PageID)
	if err != nil {
		return err
	}
	copy(page.Data[:], node.Serialize(t.keySize))
	t.pool.MarkDirty(node.PageID)
	t.pool.ReleasePage(node.PageID)
	return nil
}

func (t *BTree) newPage() (types.PageID, error) {
	return t.pool.AllocatePage()
}

func (t *BTree) Search(key []byte) (uint64, error) {
	node, err := t.readNode(t.rootID)
	if err != nil {
		return 0, err
	}

	for !node.IsLeaf {
		i := findChildIdx(node.Keys, key)
		node, err = t.readNode(node.ChildIDs[i])
		if err != nil {
			return 0, err
		}
	}

	idx := findKeyIdx(node.Keys, key)
	if idx == -1 {
		return 0, ErrKeyNotFound
	}
	return node.RowIDs[idx], nil
}

func (t *BTree) Insert(key []byte, rowID uint64) error {
	root, err := t.readNode(t.rootID)
	if err != nil {
		return err
	}

	if len(root.Keys) > t.order {
		newRootID, err := t.newPage()
		if err != nil {
			return err
		}
		newRoot := NewInternalNode(newRootID)
		newRoot.ChildIDs = []types.PageID{t.rootID}

		if err := t.splitChild(newRoot, 0, root); err != nil {
			return err
		}

		t.rootID = newRootID
		if err := t.writeNode(newRoot); err != nil {
			return err
		}

		return t.insertNonFull(newRootID, key, rowID)
	}

	return t.insertNonFull(t.rootID, key, rowID)
}

func (t *BTree) insertNonFull(pageID types.PageID, key []byte, rowID uint64) error {
	node, err := t.readNode(pageID)
	if err != nil {
		return err
	}

	if node.IsLeaf {
		if idx := findKeyIdx(node.Keys, key); idx != -1 {
			return ErrDuplicateKey
		}

		i := len(node.Keys)
		node.Keys = append(node.Keys, nil)
		node.RowIDs = append(node.RowIDs, 0)
		for i > 0 && bytes.Compare(key, node.Keys[i-1]) < 0 {
			node.Keys[i] = node.Keys[i-1]
			node.RowIDs[i] = node.RowIDs[i-1]
			i--
		}
		node.Keys[i] = key
		node.RowIDs[i] = rowID
		return t.writeNode(node)
	}

	i := len(node.Keys) - 1
	for i >= 0 && bytes.Compare(key, node.Keys[i]) < 0 {
		i--
	}
	i++

	child, err := t.readNode(node.ChildIDs[i])
	if err != nil {
		return err
	}

	if len(child.Keys) > t.order {
		if err := t.splitChild(node, i, child); err != nil {
			return err
		}
		if bytes.Compare(key, node.Keys[i]) > 0 {
			i++
		}
	}

	if err := t.insertNonFull(node.ChildIDs[i], key, rowID); err != nil {
		return err
	}

	return t.writeNode(node)
}

func (t *BTree) splitChild(parent *Node, idx int, child *Node) error {
	if child.IsLeaf {
		return t.splitLeafChild(parent, idx, child)
	}
	return t.splitInternalChild(parent, idx, child)
}

func (t *BTree) splitLeafChild(parent *Node, idx int, child *Node) error {
	split := len(child.Keys) / 2

	rightID, err := t.newPage()
	if err != nil {
		return err
	}
	right := NewLeafNode(rightID)
	right.Keys = make([][]byte, len(child.Keys)-split)
	right.RowIDs = make([]uint64, len(child.Keys)-split)
	copy(right.Keys, child.Keys[split:])
	copy(right.RowIDs, child.RowIDs[split:])
	right.NextLeaf = child.NextLeaf

	child.Keys = child.Keys[:split]
	child.RowIDs = child.RowIDs[:split]
	child.NextLeaf = rightID

	promoteKey := right.Keys[0]
	parent.Keys = append(parent.Keys, nil)
	parent.ChildIDs = append(parent.ChildIDs, 0)
	copy(parent.Keys[idx+1:], parent.Keys[idx:])
	copy(parent.ChildIDs[idx+2:], parent.ChildIDs[idx+1:])
	parent.Keys[idx] = promoteKey
	parent.ChildIDs[idx+1] = rightID

	if err := t.writeNode(child); err != nil {
		return err
	}
	return t.writeNode(right)
}

func (t *BTree) splitInternalChild(parent *Node, idx int, child *Node) error {
	split := len(child.Keys) / 2
	medianKey := child.Keys[split]

	rightID, err := t.newPage()
	if err != nil {
		return err
	}
	right := NewInternalNode(rightID)
	right.Keys = make([][]byte, len(child.Keys)-split-1)
	right.ChildIDs = make([]types.PageID, len(child.Keys)-split)
	copy(right.Keys, child.Keys[split+1:])
	copy(right.ChildIDs, child.ChildIDs[split+1:])

	child.Keys = child.Keys[:split]
	child.ChildIDs = child.ChildIDs[:split+1]

	parent.Keys = append(parent.Keys, nil)
	parent.ChildIDs = append(parent.ChildIDs, 0)
	copy(parent.Keys[idx+1:], parent.Keys[idx:])
	copy(parent.ChildIDs[idx+2:], parent.ChildIDs[idx+1:])
	parent.Keys[idx] = medianKey
	parent.ChildIDs[idx+1] = rightID

	if err := t.writeNode(child); err != nil {
		return err
	}
	return t.writeNode(right)
}

func (t *BTree) Delete(key []byte) error {
	root, err := t.readNode(t.rootID)
	if err != nil {
		return err
	}

	if err := t.deleteFromNode(root, key); err != nil {
		return err
	}

	root, err = t.readNode(t.rootID)
	if err != nil {
		return err
	}

	if !root.IsLeaf && len(root.Keys) == 0 {
		t.rootID = root.ChildIDs[0]
	}

	return nil
}

func (t *BTree) deleteFromNode(node *Node, key []byte) error {
	if node.IsLeaf {
		return t.deleteFromLeaf(node, key)
	}

	i := len(node.Keys) - 1
	for i >= 0 && bytes.Compare(key, node.Keys[i]) < 0 {
		i--
	}
	i++

	child, err := t.readNode(node.ChildIDs[i])
	if err != nil {
		return err
	}

	if err := t.deleteFromNode(child, key); err != nil {
		return err
	}

	child, err = t.readNode(node.ChildIDs[i])
	if err != nil {
		return err
	}

	minKeys := t.order / 2
	if len(child.Keys) < minKeys {
		if err := t.handleUnderflow(node, i); err != nil {
			return err
		}
	}

	return t.writeNode(node)
}

func (t *BTree) deleteFromLeaf(node *Node, key []byte) error {
	idx := findKeyIdx(node.Keys, key)
	if idx == -1 {
		return ErrKeyNotFound
	}

	node.Keys = append(node.Keys[:idx], node.Keys[idx+1:]...)
	node.RowIDs = append(node.RowIDs[:idx], node.RowIDs[idx+1:]...)

	return t.writeNode(node)
}

func (t *BTree) handleUnderflow(parent *Node, childIdx int) error {
	if childIdx > 0 {
		leftSibling, err := t.readNode(parent.ChildIDs[childIdx-1])
		if err != nil {
			return err
		}
		minKeys := t.order / 2
		if len(leftSibling.Keys) > minKeys {
			return t.borrowFromLeft(parent, childIdx-1, childIdx)
		}
	}

	if childIdx < len(parent.Keys) {
		rightSibling, err := t.readNode(parent.ChildIDs[childIdx+1])
		if err != nil {
			return err
		}
		minKeys := t.order / 2
		if len(rightSibling.Keys) > minKeys {
			return t.borrowFromRight(parent, childIdx, childIdx+1)
		}
	}

	if childIdx < len(parent.Keys) {
		return t.mergeRight(parent, childIdx)
	}
	return t.mergeRight(parent, childIdx-1)
}

func (t *BTree) borrowFromLeft(parent *Node, leftIdx, rightIdx int) error {
	left, err := t.readNode(parent.ChildIDs[leftIdx])
	if err != nil {
		return err
	}
	right, err := t.readNode(parent.ChildIDs[rightIdx])
	if err != nil {
		return err
	}

	if left.IsLeaf {
		right.Keys = append([][]byte{left.Keys[len(left.Keys)-1]}, right.Keys...)
		right.RowIDs = append([]uint64{left.RowIDs[len(left.RowIDs)-1]}, right.RowIDs...)
		left.Keys = left.Keys[:len(left.Keys)-1]
		left.RowIDs = left.RowIDs[:len(left.RowIDs)-1]
		parent.Keys[leftIdx] = right.Keys[0]
	} else {
		right.Keys = append([][]byte{parent.Keys[leftIdx]}, right.Keys...)
		right.ChildIDs = append([]types.PageID{left.ChildIDs[len(left.Keys)]}, right.ChildIDs...)
		parent.Keys[leftIdx] = left.Keys[len(left.Keys)-1]
		left.Keys = left.Keys[:len(left.Keys)-1]
		left.ChildIDs = left.ChildIDs[:len(left.ChildIDs)-1]
	}

	if err := t.writeNode(left); err != nil {
		return err
	}
	if err := t.writeNode(right); err != nil {
		return err
	}
	return t.writeNode(parent)
}

func (t *BTree) borrowFromRight(parent *Node, leftIdx, rightIdx int) error {
	left, err := t.readNode(parent.ChildIDs[leftIdx])
	if err != nil {
		return err
	}
	right, err := t.readNode(parent.ChildIDs[rightIdx])
	if err != nil {
		return err
	}

	if left.IsLeaf {
		left.Keys = append(left.Keys, right.Keys[0])
		left.RowIDs = append(left.RowIDs, right.RowIDs[0])
		right.Keys = right.Keys[1:]
		right.RowIDs = right.RowIDs[1:]
		parent.Keys[leftIdx] = right.Keys[0]
	} else {
		left.Keys = append(left.Keys, parent.Keys[leftIdx])
		left.ChildIDs = append(left.ChildIDs, right.ChildIDs[0])
		parent.Keys[leftIdx] = right.Keys[0]
		right.Keys = right.Keys[1:]
		right.ChildIDs = right.ChildIDs[1:]
	}

	if err := t.writeNode(left); err != nil {
		return err
	}
	if err := t.writeNode(right); err != nil {
		return err
	}
	return t.writeNode(parent)
}

func (t *BTree) mergeRight(parent *Node, leftIdx int) error {
	left, err := t.readNode(parent.ChildIDs[leftIdx])
	if err != nil {
		return err
	}
	right, err := t.readNode(parent.ChildIDs[leftIdx+1])
	if err != nil {
		return err
	}

	if left.IsLeaf {
		left.Keys = append(left.Keys, right.Keys...)
		left.RowIDs = append(left.RowIDs, right.RowIDs...)
		left.NextLeaf = right.NextLeaf
	} else {
		left.Keys = append(left.Keys, parent.Keys[leftIdx])
		left.Keys = append(left.Keys, right.Keys...)
		left.ChildIDs = append(left.ChildIDs, right.ChildIDs...)
	}

	parent.Keys = append(parent.Keys[:leftIdx], parent.Keys[leftIdx+1:]...)
	parent.ChildIDs = append(parent.ChildIDs[:leftIdx+1], parent.ChildIDs[leftIdx+2:]...)

	if err := t.writeNode(left); err != nil {
		return err
	}
	return t.writeNode(parent)
}

func (t *BTree) Scan(from, to []byte) *IndexScanner {
	return NewIndexScanner(t, from, to)
}

func findChildIdx(keys [][]byte, key []byte) int {
	lo, hi := 0, len(keys)
	for lo < hi {
		mid := (lo + hi) / 2
		if bytes.Compare(keys[mid], key) <= 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

func findKeyIdx(keys [][]byte, key []byte) int {
	lo, hi := 0, len(keys)
	for lo < hi {
		mid := (lo + hi) / 2
		cmp := bytes.Compare(keys[mid], key)
		if cmp == 0 {
			return mid
		} else if cmp < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return -1
}
