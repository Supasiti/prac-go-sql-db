package index

import "bytes"

type IndexScanner struct {
	tree    *BTree
	current *Node
	idx     int
	from    []byte
	to      []byte
}

func NewIndexScanner(tree *BTree, from, to []byte) *IndexScanner {
	s := &IndexScanner{
		tree: tree,
		from: from,
		to:   to,
	}

	node, err := tree.readNode(tree.rootID)
	if err != nil {
		return s
	}

	for !node.IsLeaf {
		i := findChildIdx(node.Keys, from)
		node, err = tree.readNode(node.ChildIDs[i])
		if err != nil {
			return s
		}
	}

	s.current = node
	s.idx = 0

	if from != nil {
		for s.idx < len(s.current.Keys) && bytes.Compare(s.current.Keys[s.idx], from) < 0 {
			s.idx++
		}
	}

	return s
}

func (s *IndexScanner) Next() ([]byte, uint64, bool) {
	for s.current != nil {
		if s.idx < len(s.current.Keys) {
			key := s.current.Keys[s.idx]
			if s.to != nil && bytes.Compare(key, s.to) > 0 {
				return nil, 0, false
			}
			rowID := s.current.RowIDs[s.idx]
			s.idx++
			return key, rowID, true
		}

		if s.current.NextLeaf == 0 {
			s.current = nil
			return nil, 0, false
		}

		node, err := s.tree.readNode(s.current.NextLeaf)
		if err != nil {
			s.current = nil
			return nil, 0, false
		}
		s.current = node
		s.idx = 0
	}

	return nil, 0, false
}

func (s *IndexScanner) Close() error {
	s.current = nil
	return nil
}
