package avlmerkle

type Iterator struct {
	currentNode *TreeNode
	stack       *Stack
}

type StackNode struct {
	node *TreeNode
	next *StackNode
}

type Stack struct {
	head *StackNode
}

// Push a new stackNode onto the stack
func (s *Stack) push(n *TreeNode) {
	if s.head == nil {
		s.head = &StackNode{node: n}
	} else {
		nnode := StackNode{node: n, next: s.head}
		s.head = &nnode
	}
}

// Pop the head stackNode from the stack
func (s *Stack) pop() *StackNode {
	if s.head == nil {
		return nil
	}

	ret := s.head
	s.head = s.head.next
	return ret
}

func (iter *Iterator) Next() *TreeNode {

	if iter.currentNode == nil {
		return nil
	}

	node := *iter.currentNode

	if iter.currentNode.rightNode != nil {
		cur := iter.currentNode.rightNode
		for cur.leftNode != nil {
			iter.stack.push(cur)
			cur = cur.rightNode
		}
		iter.currentNode = cur
	} else {
		if iter.stack.isEmpty() {
			iter.currentNode = nil
		} else {
			iter.currentNode = iter.stack.pop().node
		}
	}
	return &node
}
