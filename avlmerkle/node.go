// Copyright 2018 The xfsgo Authors
// This file is part of the xfsgo library.
//
// The xfsgo library is free software: you can redistribute it and/or modify
// it under the terms of the MIT Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The xfsgo library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// MIT Lesser General Public License for more details.
//
// You should have received a copy of the MIT Lesser General Public License
// along with the xfsgo library. If not, see <https://mit-license.org/>.

package avlmerkle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
)

type TreeNode struct {
	leftNode, rightNode *TreeNode
	left, right         []byte
	id, key, value      []byte
	depth               int
}

func newLeafNode(k, v []byte) *TreeNode {
	n := &TreeNode{
		key:   k,
		value: v,
		depth: 0,
	}
	n.rehash()
	return n
}
func encodeUncertainData(bs []byte, encode *[]byte) error {
	buf := bytes.NewBuffer(nil)
	if uint32(len(bs)) > uint32(math.MaxUint32) {
		return errors.New("data to long")
	}
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(bs)))
	buf.Write(lenBuf[:])
	buf.Write(bs)
	tmp := buf.Bytes()
	*encode = tmp
	return nil
}

func decodeUncertainData(buf *bytes.Buffer, data *[]byte) error {
	var lenBuf [4]byte
	if _, err := buf.Read(lenBuf[:]); err != nil {
		return err
	}
	keyLen := binary.LittleEndian.Uint32(lenBuf[:])
	*data = make([]byte, keyLen)
	if _, err := buf.Read(*data); err != nil {
		return err
	}
	return nil
}

func (n *TreeNode) Encode() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.Write([]byte{uint8(n.depth)})
	var keyEncoded []byte
	if err := encodeUncertainData(n.key, &keyEncoded); err != nil {
		return nil, errors.New("key to long")
	}
	buf.Write(keyEncoded)
	if !n.isLeaf() {
		buf.Write(n.left[:32])
		buf.Write(n.right[:32])
	} else {
		var valueEncoded []byte
		if err := encodeUncertainData(n.value, &valueEncoded); err != nil {
			return nil, errors.New("key to long")
		}
		buf.Write(valueEncoded)
	}
	return buf.Bytes(), nil
}

func (n *TreeNode) Decode(data []byte) error {

	buf := bytes.NewBuffer(data)

	if b, err := buf.ReadByte(); err == nil {
		n.depth = int(b)
	} else {
		return err
	}
	if err := decodeUncertainData(buf, &n.key); err != nil {
		return err
	}
	if !n.isLeaf() {
		n.left = make([]byte, 32)
		n.right = make([]byte, 32)
		if _, err := buf.Read(n.left); err != nil {
			return err
		}
		if _, err := buf.Read(n.right); err != nil {
			return err
		}
	} else {
		// handle val
		if err := decodeUncertainData(buf, &n.value); err != nil {
			return err
		}
	}
	n.rehash()
	return nil
}

func (n *TreeNode) rehash() {
	bs, err := rawencode.Encode(n)
	if err != nil {
		n.id = make([]byte, 32)
		return
	}
	hash := ahash.SHA256(bs)
	n.id = hash
}

func (n *TreeNode) Hash() common.Hash {
	return common.Bytes2Hash(n.id)
}

func (n *TreeNode) Value() []byte {
	return n.value
}

func (n *TreeNode) Key() []byte {
	return n.key
}

func (n *TreeNode) remove(t *Tree, key []byte) *TreeNode {
	// 找不到key对应node,返回0,nil
	if n == nil {
		return nil
	}
	var retNode *TreeNode
	// var isRemove int
	switch {
	case bytes.Compare(key, n.key) < common.Zero:
		n.leftNode = n.leftNode.remove(t, key)
		retNode = n
	case bytes.Compare(key, n.key) > common.Zero:
		n.rightNode = n.rightNode.remove(t, key)
		retNode = n
	default:
		switch {
		case n.leftNode == nil: // 待删除节点左子树为空的情况
			retNode = n.rightNode
		case n.rightNode == nil: // 待删除节点右子树为空的情况
			retNode = n.leftNode
		default:
			// 待删除节点左右子树均不为空的情况
			// 找到比待删除节点大的最小节点,即右子树的最小节点
			retNode := n.rightNode.getMinNode()
			// TODO: 这步好好理解,维护平衡性
			retNode.rightNode = n.rightNode.remove(t, key)
			retNode.left = n.left
		}
	}

	// 前面删除节点后,返回retNode有可能为空,这样在执行下面的语句会产生异常，加这步判定避免异常
	if retNode == nil {
		return retNode
	}

	retNode = retNode.rebalance(t)
	return retNode
}

// 找出以nd为根节点中最小值的节点
func (n *TreeNode) getMinNode() *TreeNode {
	if n.left == nil {
		return n
	}
	return n.leftNode.getMinNode()
}

func (n *TreeNode) insert(t *Tree, k, v []byte) *TreeNode {
	if n.isLeaf() {
		// If the key values are consistent, update the value
		if bytes.Compare(k, n.key) == common.Zero {
			return n.update(func(node *TreeNode) {
				node.value = v
			})
		}
		out := n.update(func(node *TreeNode) {
			// If the specified K is less than node K, the new node is on the left and the original node is on the right
			if bytes.Compare(k, n.key) < common.Zero {
				newLeft := newLeafNode(k, v)
				node.left = newLeft.id
				node.leftNode = newLeft
				node.right = n.id
				node.rightNode = n
			} else { // else，The new node is on the right and the original node is on the left
				node.left = n.id
				node.leftNode = n
				newRight := newLeafNode(k, v)
				node.right = newRight.id
				node.rightNode = newRight
			}
			// Synchronize nodes to tree
			node.sync(t, nil, nil)
		})
		return out
	}

	leftNode := t.mustLoadLeft(n)
	rightNode := t.mustLoadRight(n)

	if bytes.Compare(k, leftNode.key) <= common.Zero {
		return n.update(func(node *TreeNode) {
			leftNode = leftNode.insert(t, k, v)
			node.left = leftNode.id
			node.leftNode = leftNode
			node.sync(t, leftNode, rightNode)
		}).rebalance(t)
	}
	return n.update(func(node *TreeNode) {
		rightNode = rightNode.insert(t, k, v)
		node.right = rightNode.id
		node.rightNode = rightNode
		node.sync(t, leftNode, rightNode)
	}).rebalance(t)
}

func (n *TreeNode) lookup(t *Tree, k []byte) ([]byte, bool) {
	// Judge whether the current node is a leaf node
	if n.isLeaf() {
		// If the specified key is the current node, the current node value is returned directly
		if bytes.Compare(k, n.key) == common.Zero {
			return n.value, true
		}
		return nil, false
	}
	child := t.mustLoadLeft(n)
	if bytes.Compare(k, child.key) <= common.Zero {
		return child.lookup(t, k)
	}
	return t.mustLoadRight(n).lookup(t, k)
}

func (n *TreeNode) balanceFactor(t *Tree, left *TreeNode, right *TreeNode) int {
	if left == nil {
		left = t.mustLoadLeft(n)
	}

	if right == nil {
		right = t.mustLoadRight(n)
	}

	return left.depth - right.depth
}
func (n *TreeNode) leftRotate(t *Tree) *TreeNode {
	right := t.mustLoadRight(n)

	nn := n.update(func(node *TreeNode) {
		node.right = right.left
		node.rightNode = right.leftNode
		node.sync(t, nil, nil)
	})
	*n = *nn
	right = right.update(func(node *TreeNode) {
		node.left = n.id
		node.leftNode = n
		node.sync(t, nil, nil)
	})

	return right
}

func (n *TreeNode) rightRotate(t *Tree) *TreeNode {
	left := t.mustLoadLeft(n)

	nn := n.update(func(node *TreeNode) {
		node.left = left.right
		node.leftNode = left.rightNode
		node.sync(t, nil, nil)
	})
	*n = *nn
	left = left.update(func(node *TreeNode) {
		node.right = n.id
		node.rightNode = n
		node.sync(t, nil, nil)
	})

	return left
}

func (n *TreeNode) rebalance(tree *Tree) *TreeNode {
	left := tree.mustLoadLeft(n)
	right := tree.mustLoadRight(n)
	balance := n.balanceFactor(tree, left, right)
	if balance > 1 {
		if left.balanceFactor(tree, nil, nil) < 0 {
			nn := n.update(func(node *TreeNode) {
				newLeft := left.leftRotate(tree)
				node.left = newLeft.id
				node.leftNode = newLeft
			})
			*n = *nn
		}

		return n.rightRotate(tree)
	} else if balance < -1 {
		if right.balanceFactor(tree, nil, nil) > 0 {
			nn := n.update(func(node *TreeNode) {
				newRight := right.rightRotate(tree)
				node.right = newRight.id
				node.rightNode = newRight
			})
			*n = *nn
		}

		return n.leftRotate(tree)
	}
	return n
}

func (n *TreeNode) sync(tree *Tree, left, right *TreeNode) {
	if left == nil {
		// Load left node
		left = tree.mustLoadLeft(n)
	}
	if right == nil {
		// Load right node
		right = tree.mustLoadRight(n)
	}
	if left.depth > right.depth {
		n.depth = left.depth + 1
	} else {
		n.depth = right.depth + 1
	}
	if bytes.Compare(left.key, right.key) > common.Zero {
		n.key = left.key
	} else {
		n.key = right.key
	}
}

func (n *TreeNode) isLeaf() bool {
	return n.depth == 0
}

func (n *TreeNode) clone() *TreeNode {
	cloned := *n
	return &cloned
}

func (n *TreeNode) update(fn func(node *TreeNode)) *TreeNode {
	cpy := n.clone()
	fn(cpy)
	cpy.rehash()
	return cpy
}

func (n *TreeNode) dfsCall(t *Tree, fn func(node *TreeNode) error) error {
	if err := fn(n); err != nil {
		return err
	}
	if n.isLeaf() {
		return nil
	}
	left, err := t.loadLeft(n)
	if err != nil {
		return err
	}
	if err = left.dfsCall(t, fn); err != nil {
		return err
	}
	right, err := t.loadRight(n)
	if err != nil {
		return err
	}
	if err = right.dfsCall(t, fn); err != nil {
		return err
	}
	return nil
}
