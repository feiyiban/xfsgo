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

package xfsgo

import (
	"encoding/binary"
	"xfsgo/common"
	"xfsgo/common/rawencode"
	"xfsgo/storage/badger"

	"github.com/sirupsen/logrus"
)

var (
	blockHashPre       = []byte("bh:")
	blockHeightPre     = []byte("bn:")
	blockHeightHashPre = []byte("bnh:")
	lastBlockKey       = []byte("LastBlock")
)

type chainDB struct {
	storage badger.IStorage
	debug   bool
}

func newChainDBN(db badger.IStorage, debug bool) *chainDB {
	tdb := &chainDB{
		storage: db,
		debug:   debug,
	}
	return tdb
}

// Get blockHeader from hash
func (db *chainDB) GetBlockHeaderByHash(hash common.Hash) *BlockHeader {
	key := append(blockHashPre, hash.Bytes()...)
	val, err := db.storage.GetData(key)
	if err != nil {
		return nil
	}
	blockHeader := &BlockHeader{}

	if err := rawencode.Decode(val, blockHeader); err != nil {
		logrus.Debugf("Decode:%v", err)
		return nil
	}
	return blockHeader
}

// Get blockHeader from height
func (db *chainDB) GetBlockHeaderByHeight(height uint64) *BlockHeader {
	var numBuf [8]byte
	binary.LittleEndian.PutUint64(numBuf[:], height)
	key := append(blockHeightPre, numBuf[:]...)
	val, err := db.storage.GetData(key)
	if err != nil {
		return nil
	}
	hash := common.Bytes2Hash(val)
	return db.GetBlockHeaderByHash(hash)
}

// Get Current Optimum height's BlockHeader
func (db *chainDB) GetOptimumHeightBHeader() *BlockHeader {
	val, err := db.storage.GetData(lastBlockKey)
	if err != nil {
		return nil
	}
	hash := common.Bytes2Hash(val)
	return db.GetBlockHeaderByHash(hash)
}

// GetBlockHeaderByHashAndHeight
func (db *chainDB) GetBlocksByHashAndHeight(headerHash common.Hash, height uint64) *BlockHeader {
	var heightbytes = make([]byte, 8)
	binary.BigEndian.PutUint64(heightbytes, height)
	// bnh:<height_64bits><hash>
	key := append(blockHeightHashPre, heightbytes...)
	key = append(key, headerHash[:]...)
	val, err := db.storage.GetData(key)
	if err != nil {
		logrus.Errorf("Remove blockHeader by height and hash err: %s", err)
		return nil
	}

	blockHeader := &BlockHeader{}
	if err := rawencode.Decode(val, blockHeader); err != nil {
		logrus.Debugf("GetBlocksByHashAndHeight Decode:%v", err)
		return nil
	}
	return blockHeader
}

// Write BlockHeader links Hash to chainDB
func (db *chainDB) WriteBHeaderWithHash(blockHeader *BlockHeader) error {
	hash := blockHeader.HeaderHash()
	key := append(blockHashPre, hash.Bytes()...)
	val, err := rawencode.Encode(blockHeader)
	if err != nil {
		logrus.Errorf("Write block err: %s", err)
		return err
	}
	if err = db.storage.SetData(key, val); err != nil {
		logrus.Errorf("Write block err: %s", err)
		return err
	}
	return nil
}

// Write BlockHeader'hash links height to chainDB
func (db *chainDB) WriteBHeaderHashWithHeight(height uint64, headerHash common.Hash) error {
	var numBuf [8]byte
	binary.LittleEndian.PutUint64(numBuf[:], height)
	key := append(blockHeightPre, numBuf[:]...)
	bHeaderHash := headerHash
	err := db.storage.SetData(key, bHeaderHash.Bytes())
	if err != nil {
		logrus.Errorf("Write canon number err: %s", err)
		return err
	}
	return nil
}

// Write BlockHeader links height and hash to chainDB
func (db *chainDB) WriteBHeaderWithHeightAndHash(blockHeader *BlockHeader) error {
	height := blockHeader.Height
	var heightbytes = make([]byte, 8)
	binary.BigEndian.PutUint64(heightbytes, height)
	hash := blockHeader.HeaderHash()
	// bnh:<height_64bits><hash>
	key := append(blockHeightHashPre, heightbytes...)
	key = append(key, hash[:]...)
	val, err := rawencode.Encode(blockHeader)
	if err != nil {
		logrus.Errorf("Write blockHeader with number and hash err: %s", err)
		return err
	}
	if err = db.storage.SetData(key, val); err != nil {
		logrus.Errorf("Write blockHeader with number and hash err: %s", err)
		return err
	}
	return nil
}

// WriteLastBHash Write Optimum height's BlockHeader with hash
func (db *chainDB) WriteLastBHash(bHash common.Hash) error {
	if err := db.storage.SetData(lastBlockKey, bHash.Bytes()); err != nil {
		logrus.Errorf("Write hash of chain's last blockheader err: %s", err)
		return err
	}
	return nil
}

// WriteBHeader2Chain write BlockHeader of block to chainDB
func (db *chainDB) WriteBHeader2Chain(blockHeader *BlockHeader) error {
	if err := db.WriteBHeaderHashWithHeight(blockHeader.Height, blockHeader.HeaderHash()); err != nil {
		return err
	}

	if err := db.WriteLastBHash(blockHeader.HeaderHash()); err != nil {
		return err
	}
	return nil
}

//DelBHeaderByHeightAndHash Del BlockHeader linked with height and hash by height and hash
func (db *chainDB) DelBHeaderByHeightAndHash(height uint64, hash common.Hash) error {
	var heightbytes = make([]byte, 8)
	binary.BigEndian.PutUint64(heightbytes, height)
	// bnh:<height_64bits><hash>
	key := append(blockHeightHashPre, heightbytes...)
	key = append(key, hash[:]...)
	if err := db.storage.DelData(key); err != nil {
		logrus.Errorf("Remove blockHeader by height and hash err: %s", err)
		return err
	}
	return nil
}

// DelBHeaderByHash Del BlockHeader linked with Hash by Hash
func (db *chainDB) DelBHeaderByBHash(hash common.Hash) error {
	key := append(blockHashPre, hash.Bytes()...)
	if err := db.storage.DelData(key); err != nil {
		logrus.Errorf("Remove blockHeader linked with Hash by Hash err: %s", err)
		return err
	}
	return nil
}
