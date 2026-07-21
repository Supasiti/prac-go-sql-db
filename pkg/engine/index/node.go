package index

import (
	"encoding/binary"

	"github.com/tharatornsupasiti/prac-go-sql-db/pkg/types"
)

const (
	leafHeaderSize     = 16 // 1+2+8+5 (isLeaf, numKeys, nextLeaf, pad)
	internalHeaderSize = 11 // 1+2+8 (isLeaf, numKeys, reserved)
	keyLenSize         = 2
	rowIDSize          = 8
	childPtrSize       = 8
)

type Node struct {
	IsLeaf   bool
	Keys     [][]byte
	ChildIDs []types.PageID
	RowIDs   []uint64
	NextLeaf types.PageID
	PageID   types.PageID
}

func NewLeafNode(pageID types.PageID) *Node {
	return &Node{
		IsLeaf:   true,
		PageID:   pageID,
		NextLeaf: 0,
	}
}

func NewInternalNode(pageID types.PageID) *Node {
	return &Node{
		IsLeaf: false,
		PageID: pageID,
	}
}

func (n *Node) Serialize(keySize int) []byte {
	data := make([]byte, types.PageSize)

	if n.IsLeaf {
		data[0] = 1
		binary.LittleEndian.PutUint16(data[1:3], uint16(len(n.Keys)))
		binary.LittleEndian.PutUint64(data[3:11], uint64(n.NextLeaf))

		offset := leafHeaderSize
		for i, key := range n.Keys {
			binary.LittleEndian.PutUint16(data[offset:offset+keyLenSize], uint16(len(key)))
			offset += keyLenSize
			copy(data[offset:offset+len(key)], key)
			offset += len(key)
			binary.LittleEndian.PutUint64(data[offset:offset+rowIDSize], n.RowIDs[i])
			offset += rowIDSize
		}
	} else {
		data[0] = 0
		binary.LittleEndian.PutUint16(data[1:3], uint16(len(n.Keys)))

		offset := internalHeaderSize
		for i, key := range n.Keys {
			binary.LittleEndian.PutUint16(data[offset:offset+keyLenSize], uint16(len(key)))
			offset += keyLenSize
			copy(data[offset:offset+len(key)], key)
			offset += len(key)
			binary.LittleEndian.PutUint64(data[offset:offset+childPtrSize], uint64(n.ChildIDs[i]))
			offset += childPtrSize
		}
		binary.LittleEndian.PutUint64(data[offset:offset+childPtrSize], uint64(n.ChildIDs[len(n.Keys)]))
	}

	return data
}

func Deserialize(data []byte, pageID types.PageID) *Node {
	isLeaf := data[0] == 1
	numKeys := int(binary.LittleEndian.Uint16(data[1:3]))

	n := &Node{
		IsLeaf: isLeaf,
		PageID: pageID,
		Keys:   make([][]byte, numKeys),
	}

	if isLeaf {
		n.NextLeaf = types.PageID(binary.LittleEndian.Uint64(data[3:11]))
		n.RowIDs = make([]uint64, numKeys)

		offset := leafHeaderSize
		for i := 0; i < numKeys; i++ {
			keyLen := int(binary.LittleEndian.Uint16(data[offset : offset+keyLenSize]))
			offset += keyLenSize
			n.Keys[i] = make([]byte, keyLen)
			copy(n.Keys[i], data[offset:offset+keyLen])
			offset += keyLen
			n.RowIDs[i] = binary.LittleEndian.Uint64(data[offset : offset+rowIDSize])
			offset += rowIDSize
		}
	} else {
		n.ChildIDs = make([]types.PageID, numKeys+1)

		offset := internalHeaderSize
		for i := 0; i < numKeys; i++ {
			keyLen := int(binary.LittleEndian.Uint16(data[offset : offset+keyLenSize]))
			offset += keyLenSize
			n.Keys[i] = make([]byte, keyLen)
			copy(n.Keys[i], data[offset:offset+keyLen])
			offset += keyLen
			n.ChildIDs[i] = types.PageID(binary.LittleEndian.Uint64(data[offset : offset+childPtrSize]))
			offset += childPtrSize
		}
		n.ChildIDs[numKeys] = types.PageID(binary.LittleEndian.Uint64(data[offset : offset+childPtrSize]))
	}

	return n
}
