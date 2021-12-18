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
	"encoding/hex"
	"errors"
	"xfsgo/common"
	"xfsgo/common/rawencode"
	"xfsgo/lru"
	"xfsgo/storage/badger"

	// "github.com/btcsuite/goleveldb/leveldb/journal"

	"github.com/sirupsen/logrus"
)

type Tree struct {
	db     *treeDb
	root   *TreeNode
	cache  *lru.Cache
	prefix []byte
}

// NewTree creates a trie with an existing db and a root node.
// If the root exists and its format is correct, you need load the root node from the db
// and store the datas in a cache.
func NewTree(db badger.IStorage, root []byte, prefix []byte) *Tree {
	t := &Tree{
		db:     newTreeDb(db),
		prefix: make([]byte, 0),
	}

	if len(prefix) > 0 {
		t.prefix = prefix
	}
	t.cache = lru.NewCache(2048)
	var zero [32]byte
	if root != nil && len(root) == 32 && bytes.Compare(root, zero[:]) > common.Zero {
		t.root = t.mustLoadNode(root)
	}

	return t
}
func NewTreeN(db badger.IStorage, root []byte, prefix []byte) (*Tree, error) {
	var err error
	t := &Tree{
		db:     newTreeDb(db),
		prefix: make([]byte, 0),
	}

	if len(prefix) > 0 {
		t.prefix = prefix
	}
	t.cache = lru.NewCache(2048)
	var zero [32]byte
	if root != nil && len(root) == 32 && bytes.Compare(root, zero[:]) > common.Zero {
		t.root, err = t.loadNode(root)
		if err != nil {
			logrus.Errorf("Faild load tree: root=%x", root[len(root)-4:])
			return nil, err
		}
	}

	return t, nil
}

func (t *Tree) Put(k, v []byte) {
	if t.root == nil {
		t.root = newLeafNode(k, v)
		return
	}

	if len(t.prefix) > 0 {
		k = append(k, t.prefix...)
	}

	t.root = t.root.insert(t, k, v)
}

func (t *Tree) Remove(k []byte) error {
	if t.root == nil {
		return errors.New("failed to remove,avlTree is empty.")
	}
	if len(t.prefix) > 0 {
		k = append(k, t.prefix...)
	}
	t.root = t.root.remove(t, k)
	return nil
}

func (t *Tree) Update(k, v []byte) error {
	if t.root == nil {
		return errors.New("root not nil")
	}
	if len(t.prefix) > 0 {
		k = append(k, t.prefix...)
	}
	t.root = t.root.insert(t, k, v)
	return nil
}

func (t *Tree) Copy() *Tree {
	return &Tree{db: t.db, root: t.root, cache: t.cache}
}

func (t *Tree) Hash() common.Hash {
	return t.root.Hash()
}

func (t *Tree) Checksum() []byte {
	if t.root == nil {
		return nil
	}
	return t.root.id
}

func (t *Tree) ChecksumHex() string {
	bs := t.Checksum()
	if bs == nil {
		return ""
	}
	return hex.EncodeToString(bs)
}

func (t *Tree) mustLoadLeft(node *TreeNode) *TreeNode {
	if node.leftNode != nil {
		return node.leftNode
	}
	ret := t.mustLoadNode(node.left)
	node.leftNode = ret
	return ret
}

func (t *Tree) mustLoadRight(node *TreeNode) *TreeNode {
	if node.rightNode != nil {
		return node.rightNode
	}
	ret := t.mustLoadNode(node.right)
	node.rightNode = ret
	return ret
}

func (t *Tree) mustLoadNode(id []byte) *TreeNode {
	n, err := t.loadNode(id)
	if err != nil {
		panic(err)
	}
	return n
}

func (t *Tree) loadLeft(n *TreeNode) (*TreeNode, error) {
	if n.leftNode != nil {
		return n.leftNode, nil
	}

	ret, err := t.loadNode(n.left)
	if err != nil {
		return nil, err
	}

	n.leftNode = ret

	return ret, nil
}
func (t *Tree) loadRight(n *TreeNode) (*TreeNode, error) {
	if n.rightNode != nil {
		return n.rightNode, nil
	}

	ret, err := t.loadNode(n.right)
	if err != nil {
		return nil, err
	}

	n.rightNode = ret

	return ret, nil
}

func (t *Tree) loadNode(id []byte) (*TreeNode, error) {
	var zero [32]byte
	if bytes.Compare(zero[:], id) == common.Zero {
		return nil, nil
	}
	var mId [32]byte
	copy(mId[:], id)
	if data, has := t.cache.Get(mId); has {
		tn := &TreeNode{}
		if err := rawencode.Decode(data, tn); err != nil {
			return nil, err
		}
		return tn, nil
	}
	tn, err := t.db.getTreeNodeByKey(append([]byte("tree:"), id...))
	if err != nil {
		return nil, err
	}
	// push cache
	buf, err := rawencode.Encode(tn)
	if err != nil {
		return nil, err
	}
	t.cache.Put(mId, buf)
	return tn, nil
}

// Get returns the value for key stored in the trie.
// The value bytes must not be modified by the caller.
func (t *Tree) Get(k []byte) ([]byte, bool) {
	if t.root == nil {
		return nil, false
	}
	if len(t.prefix) > 0 {
		k = append(k, t.prefix...)
	}
	return t.root.lookup(t, k)
}

func (t *Tree) Foreach(fn func(key []byte, value []byte)) {
	if t.root == nil {
		return
	}
	t.foreach(t.root, fn)
}

func (t *Tree) foreach(n *TreeNode, fn func(key []byte, value []byte)) {
	if n == nil {
		return
	}
	if n.isLeaf() {
		fn(n.key, n.value)
		return
	}
	t.foreach(t.mustLoadLeft(n), fn)
	t.foreach(t.mustLoadRight(n), fn)
}

func (s *Stack) isEmpty() bool {
	return s.head == nil
}

// Initializes a new iterator.
// Since there are no parent pointers in the tree currently,
// a stack is being use to traverse in-order.
func (t *Tree) NewIterator(prefix []byte) *Iterator {
	if t.root == nil {
		return nil
	}
	iter := Iterator{stack: &Stack{}}
	cur := t.root
	for cur.leftNode != nil {
		if len(prefix) > 0 {
			key := append(t.prefix, prefix...)
			if common.BytesEquals(key, cur.leftNode.key) {
				iter.stack.push(cur)
				cur = cur.leftNode
			}
		} else {
			iter.stack.push(cur)
			cur = cur.leftNode
		}
	}
	iter.currentNode = cur
	return &iter
}

func (t *Tree) Commit() error {
	if t.root == nil {
		return nil
	}

	batch := t.db.newWriteBatch()
	err := t.root.dfsCall(t, func(node *TreeNode) error {
		root := t.Checksum()
		_ = root
		key := append([]byte("tree:"), node.id...)

		bs, err := rawencode.Encode(node)
		if err != nil {
			return err
		}
		// return batch.Put(append([]byte("tree:"), node.id...), bs)
		if len(root) == 0 || len(key) == 0 || len(bs) == 0 {
			return errors.New("root or key or bs not null")
		}
		return batch.Put(key, bs)
	})
	if err != nil {
		return err
	}
	if err = t.db.commitBatch(batch); err != nil {
		return err
	}
	return nil
}
