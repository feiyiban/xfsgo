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
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
	"xfsgo/common"
	"xfsgo/params"
	"xfsgo/storage/badger"
	"xfsgo/vm/evm"

	"github.com/sirupsen/logrus"
)

var zeroBigN = new(big.Int).SetInt64(0)

const (
	// blocks can be created per second(in seconds)
	// adjustment factor
	adjustmentFactor    = int64(2)
	maxOrphanBlocks     = 100
	targetTimePerBlock  = int64(time.Minute * 3 / time.Second)
	targetTimespanPreV4 = int64(time.Hour * 1 / time.Second)
	targetTimespanV4    = int64(time.Minute * 57 / time.Second)
	endTimeV2           = int64(time.Hour * 1008 / time.Second)
	endTimeV3           = int64(time.Hour * 168 / time.Second)
	endTimeV4           = int64(time.Hour * 538 / time.Second)
	totalblocksv2       = endTimeV2 / targetTimePerBlock
	totalblocksv3       = endTimeV3 / targetTimePerBlock
	totalblocksv4       = endTimeV4 / targetTimePerBlock
	totalblocks         = endTimeV2/targetTimePerBlock + endTimeV3/targetTimePerBlock + endTimeV4/targetTimePerBlock
	//targetTimePerBlock = int64(time.Minute * 1 / time.Second)
	//targetTimespan  = int64(time.Minute * 10 / time.Second)
	//endTimeV1 = int64(time.Minute * 10 / time.Second)
)

var (
	baseSubsidy, _        = new(big.Int).SetString("93755722410000000000", 10)
	baseTestSubsidy, _    = common.BaseCoin2Atto("14")
	GasPoolOutErr         = errors.New("gas pool out err")
	ErrBlockIgnored       = errors.New("block hash ignore")
	ErrBadBlock           = errors.New("bad block")
	ErrApplyTransactions  = errors.New("apply transactions err")
	ErrWriteBlock         = errors.New("write block err")
	ErrOrphansBlock       = errors.New("block is orphans")
	ErrDifficultyOverflow = errors.New("difficulty overflow")
)

type orphanBlock struct {
	block  *Block
	expire time.Time
}
type IBlockChain interface {
	GetNonce(addr common.Address) uint64
	GetBlockByNumber(num uint64) *Block
	getBlockByNumber(num uint64) *Block
	GetBlockHeaderByBHash(hash common.Hash) *BlockHeader
	GetBlockByHash(hash common.Hash) *Block
	GetBlockByHashWithoutRec(hash common.Hash) *Block
	GetReceiptByHash(hash common.Hash) *Receipt
	GetReceiptByHashIndex(hash common.Hash) *TxIndex
	// GetBlocksFromNumber(num uint64) []*Block
	GetBlocksFromHash(hash common.Hash, n int) []*Block
	GenesisBHeader() *BlockHeader
	CurrentBHeader() *BlockHeader
	LatestGasLimit() *big.Int
	LastBlockHash() common.Hash
	setLastState() error
	GetBlockReceiptsByBHash(Hash common.Hash) []*Receipt
	GetTransactionByTxHash(Hash common.Hash) *Transaction
	GetBlockTransactionsByBHash(Hash common.Hash) []*Transaction
	GetHead() *Block
	GetBalance(addr common.Address) *big.Int
	WriteBlock(block *Block) error
	writeBlock(block *Block) error
	WriteReceipts2ExtraDB(bHash common.Hash, receipts []*Receipt) error
	WriteTransactions2ExtraDB(bHash common.Hash, height uint64, transactions []*Transaction) error
	WriteBHeader2ChainDBWithHash(blockHeader *BlockHeader) error
	WriteBHeader2Chain(blockHeader *BlockHeader) error
	DelReceipts(receipts []*Receipt) error
	DelBlockReceiptsByBHash(hash common.Hash) error
	DelBlockReceiptsByBHashEx(bHash common.Hash, receipts []*Receipt) error
	DelTransactionByTxHash(hash common.Hash) error
	DelBlockTransactionsByBHashEx(bHash common.Hash, height uint64, transactions []*Transaction) error
	DelBHeaderByBHash(hash common.Hash) error
	MaybeAcceptBlock(block *Block) error
	Boundaries() (uint64, uint64)
	SetBoundaries(syncStatsOrigin, syncStatsHeight uint64) error
	InsertChain(block *Block) error
	// ApplyTransactions(stateTree *StateTree, header *BlockHeader, txs []*Transaction) (*big.Int, []*Receipt, error)
	// ApplyTransaction(stateTree *StateTree, _ *BlockHeader, tx *Transaction, gp *GasPool, totalGas *big.Int) (*Receipt, error)
	IntrinsicGas(data []byte) *big.Int
	GetBlockHashes(from uint64, count uint64) []common.Hash
	GetBlockHashesFromHash(hash common.Hash, max uint64) (chain []common.Hash)
	GetBlocks(from uint64, count uint64) []*Block
	FindAncestor(bHeader *BlockHeader, height uint64) *BlockHeader
	CalcNextRequiredDifficulty() (uint32, error)
	CalcNextRequiredBitsByHeight(height uint64) (uint32, error)
	CurrentStateTree() *StateTree
	GetHeader(hash common.Hash, number uint64) *BlockHeader
	GetVMConfig() *evm.Config
}

// BlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain inserts, saves, transfers state.
// The BlockChain also helps in returning blocks required from any chain included
// in the database as well as blocks that represents the canonical chain.
type BlockChain struct {
	chainConfig    *params.ChainConfig // Chain & network configuration
	stateDB        badger.IStorage
	chainDB        *chainDB
	extraDB        *extraDB
	genesisBHeader *BlockHeader
	currentBHeader *BlockHeader
	lastBlockHash  common.Hash
	stateTree      *StateTree
	mu             sync.RWMutex
	chainmu        sync.RWMutex
	eventBus       *EventBus
	// orphans
	orphans      map[common.Hash]*orphanBlock
	prevOrphans  map[common.Hash][]*orphanBlock
	oldestOrphan *orphanBlock
	orphanLock   sync.RWMutex
	// Statistics
	syncStatsOrigin uint64       // Origin block number where syncing started at
	syncStatsHeight uint64       // Highest block number known when syncing started
	syncStatsLock   sync.RWMutex // Lock protecting the sync stats fields

	vmConfig evm.Config
}

func NewBlockChainN(stateDB, chainDB, extraDB badger.IStorage, eventBus *EventBus, debug bool) (*BlockChain, error) {
	bc := &BlockChain{
		chainDB:  newChainDBN(chainDB, debug),
		stateDB:  stateDB,
		extraDB:  newExtraDB(extraDB),
		eventBus: eventBus,
	}
	bc.orphans = make(map[common.Hash]*orphanBlock)
	bc.prevOrphans = make(map[common.Hash][]*orphanBlock)

	genesisBlock := bc.GetBlockByNumber(0)
	if genesisBlock == nil {
		return nil, errors.New("no genesis block")
	}

	bc.genesisBHeader = genesisBlock.Header

	if err := bc.setLastState(); err != nil {
		return nil, err
	}
	stateRootHash := bc.currentBHeader.StateRoot
	bc.stateTree = NewStateTree(stateDB, stateRootHash.Bytes())
	return bc, nil
}

func (bc *BlockChain) GetNonce(addr common.Address) uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.stateTree.GetNonce(addr)
}

func (bc *BlockChain) StateAt(rootHash common.Hash) *StateTree {
	return NewStateTree(bc.stateDB, rootHash.Bytes())
}

// getBlockByNumber get Block's Info about the Optimum chain
func (bc *BlockChain) GetBlockByNumber(num uint64) *Block {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.getBlockByNumber(num)
}

// getBlockByNumber get Block's Info about the Optimum chain
func (bc *BlockChain) getBlockByNumber(num uint64) *Block {
	blockHeader := bc.chainDB.GetBlockHeaderByHeight(num)
	if blockHeader == nil {
		return nil
	}

	receipts := bc.extraDB.GetBlockReceiptsByBHash(blockHeader.HeaderHash())

	transantions := bc.extraDB.GetBlockTransactionsByBHash(blockHeader.HeaderHash())

	block := &Block{Header: blockHeader, Transactions: transantions, Receipts: receipts}

	return block

}

func (bc *BlockChain) GetBlockHeaderByBHash(hash common.Hash) *BlockHeader {
	return bc.chainDB.GetBlockHeaderByHash(hash)
}

func (bc *BlockChain) GetBlockByHash(hash common.Hash) *Block {
	blockHeader := bc.chainDB.GetBlockHeaderByHash(hash)
	if blockHeader == nil {
		return nil
	}
	transactions := bc.extraDB.GetBlockTransactionsByBHash(hash)
	receipts := bc.extraDB.GetBlockReceiptsByBHash(hash)
	block := &Block{Header: blockHeader, Transactions: transactions, Receipts: receipts}

	return block
}

func (bc *BlockChain) GetBlockByHashWithoutRec(hash common.Hash) *Block {
	blockHeader := bc.chainDB.GetBlockHeaderByHash(hash)
	if blockHeader == nil {
		return nil
	}
	transactions := bc.extraDB.GetBlockTransactionsByBHash(hash)
	// receipts := bc.extraDB.GetBlockReceiptsByBHash(hash)
	block := &Block{Header: blockHeader, Transactions: transactions, Receipts: nil}

	return block
}

func (bc *BlockChain) GetReceiptByHash(hash common.Hash) *Receipt {
	return bc.extraDB.GetReceipt(hash)

}

func (bc *BlockChain) GetReceiptByHashIndex(hash common.Hash) *TxIndex {
	return bc.extraDB.GetReceiptByHashIndex(hash)
}

// func (bc *BlockChain) GetBlocksFromNumber(num uint64) []*Block {

// 	var blocks []*Block
// 	blockHeaders := bc.chainDB.GetBlocksByNumber(num)
// 	for _, v := range blockHeaders {
// 		if v == nil {
// 			continue
// 		}
// 		transactions := bc.extraDB.GetBlockTransactionsByBHash(v.HeaderHash())
// 		receipts := bc.extraDB.GetBlockReceiptsByBHash(v.HeaderHash())
// 		block := &Block{Header: v, Transactions: transactions, Receipts: receipts}
// 		blocks = append(blocks, block)
// 	}

// 	return blocks
// }

func (bc *BlockChain) GetBlocksFromHash(hash common.Hash, n int) []*Block {
	var blocks = make([]*Block, n)
	for i := 0; i < n; i++ {
		block := bc.GetBlockByHash(hash)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		hash = block.HashPrevBlock()
	}
	return blocks
}

func (bc *BlockChain) GenesisBHeader() *BlockHeader {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.genesisBHeader
}

func (bc *BlockChain) CurrentBHeader() *BlockHeader {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.currentBHeader
}
func (bc *BlockChain) LatestGasLimit() *big.Int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.currentBHeader.GasLimit
}

func (bc *BlockChain) LastBlockHash() common.Hash {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.lastBlockHash
}

func (bc *BlockChain) setLastState() error {
	if bHeader := bc.chainDB.GetOptimumHeightBHeader(); bHeader != nil {
		bc.currentBHeader = bHeader
		bc.lastBlockHash = bHeader.HeaderHash()
	}
	return nil
}

func (bc *BlockChain) GetEVM(msg Message, statedb *StateTree, header *BlockHeader) (*evm.EVM, error) {
	// Create a new context to be used in the EVM environment
	txContext := NewEVMTxContext(msg)
	blockContext := NewEVMBlockContext(header, bc, nil)
	return evm.NewEVM(blockContext, txContext, statedb, bc.vmConfig), nil
}

// GetBlockReceiptsByBHash get Receipts by blockheader hash
func (bc *BlockChain) GetBlockReceiptsByBHash(Hash common.Hash) []*Receipt {
	return bc.extraDB.GetBlockReceiptsByBHash(Hash)
}

// GetTransactionByHash get Transaction by Transaction hash
func (bc *BlockChain) GetTransactionByTxHash(Hash common.Hash) *Transaction {
	return bc.extraDB.GetTransactionByTxHash(Hash)
}

// GetBlockTransactionsByBHash get Transactions by blockheader hash
func (bc *BlockChain) GetBlockTransactionsByBHash(Hash common.Hash) []*Transaction {
	return bc.extraDB.GetBlockTransactionsByBHash(Hash)
}

func (bc *BlockChain) GetHead() *Block {
	bHeader := bc.chainDB.GetOptimumHeightBHeader()
	if bHeader == nil {
		return nil
	}

	transactions := bc.extraDB.GetBlockTransactionsByBHash(bHeader.HeaderHash())
	receipts := bc.extraDB.GetBlockReceiptsByBHash(bHeader.HeaderHash())

	block := &Block{Header: bHeader, Transactions: transactions, Receipts: receipts}

	return block
}
func (bc *BlockChain) GetBalance(addr common.Address) *big.Int {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	gotBalance := bc.stateTree.GetBalance(addr)
	return gotBalance

}

func (bc *BlockChain) WriteBlock(block *Block) error {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()

	return bc.writeBlock(block)
}

// WriteBlock stores the block inputed to the local database.
func (bc *BlockChain) writeBlock(block *Block) error {
	bc.mu.RLock()
	bHeader := bc.currentBHeader
	bc.mu.RUnlock()

	if bHeader == nil {
		return fmt.Errorf("current chain has no head")
	}

	if block.Height() > bHeader.Height {
		curHash := bHeader.HeaderHash()
		bhash := block.HeaderHash()
		prehash := block.HashPrevBlock()
		if !bytes.Equal(prehash[:], curHash[:]) {
			logrus.Debugf("Find bifurcation need reorg: blockHeight=%d, blockHash=%x, phash=%x, chianHead=%x",
				block.Height(), bhash[len(bhash)-4:], prehash[len(prehash)-4:], curHash[len(curHash)-4:])

			transactions := bc.GetBlockTransactionsByBHash(bHeader.HeaderHash())
			receipts := bc.GetBlockReceiptsByBHash(bHeader.HeaderHash())
			curBlock := &Block{Header: bHeader, Transactions: transactions, Receipts: receipts}
			if err := bc.reorg(curBlock, block); err != nil {
				return err
			}
		}
		bc.mu.Lock()
		if err := bc.insertBHeader2Chain(block.Header); err != nil {
			return err
		}
		bc.mu.Unlock()
		bc.eventBus.Publish(ChainHeadEvent{block})
	}
	if err := bc.WriteTransactions2ExtraDB(block.HeaderHash(), block.Height(), block.Transactions); err != nil {
		return err
	}
	if err := bc.WriteReceipts2ExtraDB(block.HeaderHash(), block.Receipts); err != nil {
		return err
	}

	return bc.WriteBHeader2ChainDBWithHash(block.Header)
}

func (bc *BlockChain) insertBHeader2Chain(bHeader *BlockHeader) error {
	if err := bc.WriteBHeader2Chain(bHeader); err != nil {
		logrus.Errorf("Failed insert chain: %s", err)
		return err
	}
	bc.currentBHeader = bHeader
	bc.lastBlockHash = bHeader.HeaderHash()
	lastStateRoot := bHeader.StateRoot
	bc.stateTree = NewStateTree(bc.stateDB, lastStateRoot.Bytes())
	return nil
}

func (bc *BlockChain) reorg(oldBlock, newBlock *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	var newBlocks []*Block
	var mNewBlock *Block
	//logrus.Debugf("Find newblocks: from=%d, to=%d", newBlock.Height(), oldBlock.Height())
	for mNewBlock = newBlock; mNewBlock != nil && mNewBlock.Height() != oldBlock.Height(); mNewBlock = bc.GetBlockByHash(mNewBlock.HashPrevBlock()) {
		//nhash := mNewBlock.Hash()
		//logrus.Debugf("Append newblock: height=%d, hash=%x", mNewBlock.Height(), nhash[len(nhash)-4:])
		newBlocks = append(newBlocks, newBlock)
	}
	if newBlock == nil {
		return fmt.Errorf("invalid new chain")
	}
	mOldBlock := oldBlock
	var deletedTxs []*Transaction
	var deletedBlocks []*Block
	for {
		oldhash := mOldBlock.HeaderHash()
		newhash := mNewBlock.HeaderHash()

		//logrus.Debugf("Find common hash: old=%x, new=%x", oldhash[len(oldhash)-4:], newhash[len(oldhash)-4:])
		if bytes.Equal(oldhash[:], newhash[:]) {
			break
		}
		newBlocks = append(newBlocks, mNewBlock)
		//logrus.Debugf("Found new block: height=%d, hash=%x", mNewBlock.Height(), newhash)
		//logrus.Debugf("Found old block: height=%d, hash=%x", mOldBlock.Height(), oldhash)
		deletedTxs = append(deletedTxs, mOldBlock.Transactions...)
		deletedBlocks = append(deletedBlocks, mOldBlock)
		mOldBlock = bc.GetBlockByHash(mOldBlock.HashPrevBlock())
		mNewBlock = bc.GetBlockByHash(mNewBlock.HashPrevBlock())
		if mOldBlock == nil {
			return fmt.Errorf("invalid old chain")
		}
		if mNewBlock == nil {
			return fmt.Errorf("invalid new chain")
		}
	}

	for _, block := range deletedBlocks {

		if err := bc.DelBlockReceiptsByBHashEx(block.HeaderHash(), block.Receipts); err != nil {
			return err
		}

		if err := bc.DelBlockTransactionsByBHashEx(block.HeaderHash(), block.Height(), block.Transactions); err != nil {
			return err
		}

		if err := bc.DelBHeaderByBHash(block.HeaderHash()); err != nil {
			return err
		}
	}

	var addedTxs []*Transaction
	for _, block := range newBlocks {
		_ = bc.insertBHeader2Chain(block.Header)
		//blkhash := block.Hash()
		//logrus.Debugf("Successfully new insert block: height=%d, hash: %x", block.Height(), blkhash[len(blkhash)-4:])
		// write canonical receipts and transactions
		if err := bc.WriteTransactions2ExtraDB(block.HeaderHash(), block.Height(), block.Transactions); err != nil {
			return err
		}
		if err := bc.WriteReceipts2ExtraDB(block.HeaderHash(), block.Receipts); err != nil {
			return err
		}

		addedTxs = append(addedTxs, block.Transactions...)
	}

	// Delete transactions with difference between the side chain and the main chain
	// publish these transtions to txpool
	for _, tx := range TxDifference(deletedTxs, addedTxs) {
		go bc.eventBus.Publish(TxPreEvent{Tx: tx})
		_ = bc.DelTransactionByTxHash(tx.Hash())
	}
	return nil
}

// WriteReceipts2ExtraDB write Receipts of block to extreaDB
func (bc *BlockChain) WriteReceipts2ExtraDB(bHash common.Hash, receipts []*Receipt) error {

	if err := bc.extraDB.WriteBlockReceipts(bHash, receipts); err != nil {
		return err
	}

	if err := bc.extraDB.WriteReceiptsWithRecHash(receipts); err != nil {
		return err
	}

	return nil
}

// WriteTransactions2ExtraDB write Transactions of block to extreaDB
func (bc *BlockChain) WriteTransactions2ExtraDB(bHash common.Hash, height uint64, transactions []*Transaction) error {
	if err := bc.extraDB.WriteBlockTransactionsWithBHash(bHash, transactions); err != nil {
		return err
	}

	if err := bc.extraDB.WriteBlockTransactionsWithTxIndex(bHash, height, transactions); err != nil {
		return err
	}

	if err := bc.extraDB.WriteBlockTransactionWithTxHash(transactions); err != nil {
		return err
	}

	return nil
}

// WriteBHeader2ChainDBWithHash write BlockHeader of block to chainDB
func (bc *BlockChain) WriteBHeader2ChainDBWithHash(blockHeader *BlockHeader) error {
	if err := bc.chainDB.WriteBHeaderWithHash(blockHeader); err != nil {
		return err
	}

	if err := bc.chainDB.WriteBHeaderWithHeightAndHash(blockHeader); err != nil {
		return err
	}
	return nil
}

// WriteBHeader2Chain write BlockHeader of block to chainDB
func (bc *BlockChain) WriteBHeader2Chain(blockHeader *BlockHeader) error {
	if err := bc.chainDB.WriteBHeaderHashWithHeight(blockHeader.Height, blockHeader.HeaderHash()); err != nil {
		return err
	}

	if err := bc.chainDB.WriteLastBHash(blockHeader.HeaderHash()); err != nil {
		return err
	}
	return nil
}

// DeleteReceipts removes all Receipts data associated with a block.
func (bc *BlockChain) DelReceipts(receipts []*Receipt) error {
	return bc.extraDB.DelReceipts(receipts)
}

// DelBlockReceiptsByBHash removes all Receipts data associated with a block.
func (bc *BlockChain) DelBlockReceiptsByBHash(hash common.Hash) error {
	return bc.extraDB.DelBlockReceiptsByBHash(hash)
}

// DelBlockReceiptsByBHash remove Receipts of block hash and Index from extreaDB
func (bc *BlockChain) DelBlockReceiptsByBHashEx(bHash common.Hash, receipts []*Receipt) error {
	if err := bc.extraDB.DelBlockReceiptsByBHash(bHash); err != nil {
		return err
	}

	if err := bc.extraDB.DelReceipts(receipts); err != nil {
		return err
	}

	return nil
}

// DelTransactionByTxHash removes all transaction data associated with a txhash.
func (bc *BlockChain) DelTransactionByTxHash(hash common.Hash) error {
	return bc.extraDB.DelTransactionByTxHash(hash)
}

// DelBlockTransactionsByBHash remove Transactions of block hash and Index from extreaDB
func (bc *BlockChain) DelBlockTransactionsByBHashEx(bHash common.Hash, height uint64, transactions []*Transaction) error {
	if err := bc.extraDB.DelBlockTransactionsByBHash(bHash); err != nil {
		return err
	}

	if err := bc.extraDB.DelBlockTransactionsByBIndex(bHash, height, transactions); err != nil {
		return err
	}

	return nil
}

// DelBHeaderByHash Del BlockHeader linked with Hash by Hash
func (bc *BlockChain) DelBHeaderByBHash(hash common.Hash) error {
	return bc.chainDB.DelBHeaderByBHash(hash)
}

// calculate rewards for packing the block by miners
// func calcBlockSubsidy(currentHeight uint64) *big.Int {
// 	// reduce the reward by half
// 	nSubsidy := uint64(50) >> uint(currentHeight/210000)
// 	//logrus.Debugf("nSubsidy: %d", nSubsidy)
// 	rec, _ := common.BaseCoin2Atto(strconv.FormatUint(nSubsidy, 10))
// 	return rec
// }

func calcBlockSubsidy(height uint64) *big.Int {
	if GenesisBits == TestNetGenesisBits {
		return baseTestSubsidy
	}
	return new(big.Int).Rsh(baseSubsidy, uint(height/480))
}

// AccumulateRewards calculates the rewards and add it to the miner's account.
func AccumulateRewards(stateTree *StateTree, header *BlockHeader) {
	subsidy := calcBlockSubsidy(header.Height)

	//logrus.Debugf("Current height of the blockchain %d, reward: %d", header.Height, subsidy)
	stateTree.AddBalance(header.Coinbase, subsidy)
}

func (bc *BlockChain) MaybeAcceptBlock(block *Block) error {
	return bc.maybeAcceptBlock(block)
}

func (bc *BlockChain) maybeAcceptBlock(block *Block) error {
	header := block.GetHeader()
	//blockHash := block.Hash()
	txsRoot := block.TransactionRoot()
	txs := block.Transactions
	rsRoot := block.ReceiptsRoot()
	targetTxsRoot := CalcTxsRootHash(block.Transactions)
	if !bytes.Equal(targetTxsRoot.Bytes(), txsRoot.Bytes()) {
		return ErrBadBlock
	}

	parent := bc.GetBlockByHash(block.HashPrevBlock())
	//parenthash := parent.Hash()
	parentStateRoot := parent.StateRoot()
	//logrus.Debugf("New state tree: parentHeight=%d, parentHash=%x, parentStateRoot=%x",
	//	parent.Height(), parenthash[len(blockHash)-4:], parentStateRoot[len(parentStateRoot)-4:])
	stateTree, err := NewStateTreeN(bc.stateDB, parentStateRoot.Bytes())
	if err != nil {
		logrus.Errorf("Accept block err: %v", err)
		return ErrBadBlock
	}
	gas, rec, err := bc.ApplyTransactions(stateTree, header, txs)
	if err != nil {
		logrus.Errorf("Accept block err: %v", err)
		return ErrApplyTransactions
	}
	block.Receipts = rec
	if gas.Cmp(header.GasUsed) != 0 {
		return ErrBadBlock
	}
	targetRsRoot := CalcReceiptRootHash(rec)
	if !bytes.Equal(rsRoot[:], targetRsRoot[:]) {
		return ErrBadBlock
	}
	AccumulateRewards(stateTree, header)
	stateTree.UpdateAll()
	if err = stateTree.Commit(); err != nil {
		logrus.Errorf("Accept block err: %v", err)
		return ErrWriteBlock
	}
	if err = bc.writeBlock(block); err != nil {
		logrus.Errorf("Accept block err: %v", err)
		return ErrWriteBlock
	}
	return nil
}

// Boundaries retrieves the synchronisation boundaries, specifically the origin
// block where synchronisation started at (may have failed/suspended) and the
// latest known block which the synchonisation targets.
func (bc *BlockChain) Boundaries() (uint64, uint64) {
	bc.syncStatsLock.RLock()
	defer bc.syncStatsLock.RUnlock()

	return bc.syncStatsOrigin, bc.syncStatsHeight
}

func (bc *BlockChain) SetBoundaries(syncStatsOrigin, syncStatsHeight uint64) error {
	bc.syncStatsLock.Lock()

	if bc.syncStatsHeight <= syncStatsOrigin || bc.syncStatsOrigin > syncStatsOrigin {
		bc.syncStatsOrigin = syncStatsOrigin
	}

	bc.syncStatsHeight = syncStatsHeight
	bc.syncStatsLock.Unlock()

	return nil
}

// InsertChain executes the actual chain insertion.
func (bc *BlockChain) InsertChain(block *Block) error {
	bc.chainmu.Lock()
	defer bc.chainmu.Unlock()
	blockHash := block.HeaderHash()
	header := block.GetHeader()
	//logrus.Debugf("Processing block: height=%d, hash=%x", block.Height(), blockHash[len(blockHash)-4:])
	if old := bc.GetBlockByHash(blockHash); old != nil {
		return ErrBlockIgnored
	}
	if _, exists := bc.orphans[blockHash]; exists {
		return ErrBlockIgnored
	}
	var parent *Block
	if parent = bc.GetBlockByHash(block.HashPrevBlock()); parent == nil {
		cp := block.HashPrevBlock()
		logrus.Infof("Adding orphan block: height=%d, hash=%x, prevHash=%x",
			block.Height(), blockHash[len(blockHash)-4:], cp[len(cp)-4:])
		bc.addOrphanBlock(block)
		return ErrOrphansBlock
	}
	if err := bc.checkBlockHeaderSanity(parent.Header, header, blockHash); err != nil {
		return ErrBadBlock
	}
	if err := bc.maybeAcceptBlock(block); err != nil {
		logrus.Errorf("Insert Chain err: %s", err)
		return err
	}
	if err := bc.processOrphans(blockHash); err != nil {
		logrus.Errorf("Insert Chain err: %s", err)
		return err
	}
	return nil
}
func (bc *BlockChain) removeOrphanBlock(orphan *orphanBlock) {
	bc.orphanLock.Lock()
	defer bc.orphanLock.Unlock()

	// Remove the orphan block from the orphan pool.
	orphanHash := orphan.block.HeaderHash()
	delete(bc.orphans, orphanHash)

	// Remove the reference from the previous orphan index too.  An indexing
	// for loop is intentionally used over a range here as range does not
	// reevaluate the slice on each iteration nor does it adjust the index
	// for the modified slice.
	prevHash := orphan.block.HashPrevBlock()
	orphans := bc.prevOrphans[prevHash]
	for i := 0; i < len(orphans); i++ {
		hash := orphans[i].block.HeaderHash()
		if bytes.Equal(orphanHash[:], hash[:]) {
			copy(orphans[i:], orphans[i+1:])
			orphans[len(orphans)-1] = nil
			orphans = orphans[:len(orphans)-1]
			i--
		}
	}
	bc.prevOrphans[prevHash] = orphans

	// Remove the map entry altogether if there are no longer any orphans
	// which depend on the parent hash.
	if len(bc.prevOrphans[prevHash]) == 0 {
		delete(bc.prevOrphans, prevHash)
	}
}
func (bc *BlockChain) addOrphanBlock(block *Block) {
	for _, oBlock := range bc.orphans {
		if time.Now().After(oBlock.expire) {
			bc.removeOrphanBlock(oBlock)
			continue
		}

		// Update the oldest orphan block pointer so it can be discarded
		// in case the orphan pool fills up.
		if bc.oldestOrphan == nil || oBlock.expire.Before(bc.oldestOrphan.expire) {
			bc.oldestOrphan = oBlock
		}
	}
	// Limit orphan blocks to prevent memory exhaustion.
	if len(bc.orphans)+1 > maxOrphanBlocks {
		// Remove the oldest orphan to make room for the new one.
		bc.removeOrphanBlock(bc.oldestOrphan)
		bc.oldestOrphan = nil
	}
	bc.orphanLock.Lock()
	defer bc.orphanLock.Unlock()
	expire := time.Now().Add(time.Hour)
	oBlock := &orphanBlock{
		block:  block,
		expire: expire,
	}
	hash := block.HeaderHash()
	bc.orphans[hash] = oBlock

	// Add to previous hash lookup index for faster dependency lookups.
	prevHash := block.HashPrevBlock()
	bc.prevOrphans[prevHash] = append(bc.prevOrphans[prevHash], oBlock)
}
func (bc *BlockChain) processOrphans(hash common.Hash) error {
	processHashes := make([]*common.Hash, 0, 10)
	processHashes = append(processHashes, &hash)
	for len(processHashes) > 0 {
		// Pop the first hash to process from the slice.
		processHash := processHashes[0]
		processHashes[0] = nil // Prevent GC leak.
		processHashes = processHashes[1:]

		// Look up all orphans that are parented by the block we just
		// accepted.  This will typically only be one, but it could
		// be multiple if multiple blocks are mined and broadcast
		// around the same time.  The one with the most proof of work
		// will eventually win out.  An indexing for loop is
		// intentionally used over a range here as range does not
		// reevaluate the slice on each iteration nor does it adjust the
		// index for the modified slice.
		for i := 0; i < len(bc.prevOrphans[*processHash]); i++ {
			orphan := bc.prevOrphans[*processHash][i]
			if orphan == nil {
				logrus.Warnf("Found a nil entry in the "+
					"orphan dependency list for block: index=%d, hash=%x", i,
					processHash[len(processHash)-4:])
				continue
			}

			// Remove the orphan from the orphan pool.
			orphanHash := orphan.block.HeaderHash()
			bc.removeOrphanBlock(orphan)
			i--
			// Potentially accept the block into the block chain.
			if err := bc.maybeAcceptBlock(orphan.block); err != nil {
				return err
			}
			logrus.Infof("Successfully process orphan block: hash=%x", orphanHash[len(orphanHash)-4:])
			// Add this block to the list of blocks to process so
			// any orphan blocks that depend on this block are
			// handled too.
			processHashes = append(processHashes, &orphanHash)
		}
	}
	return nil
}

// GetVMConfig returns the block chain VM config.
func (bc *BlockChain) GetVMConfig() *evm.Config {
	return &bc.vmConfig
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (bc *BlockChain) GetHeader(hash common.Hash, number uint64) *BlockHeader {
	return bc.chainDB.GetBlocksByHashAndHeight(hash, number)
}

func (bc *BlockChain) ApplyTransactions(stateTree *StateTree, header *BlockHeader, txs []*Transaction) (*big.Int, []*Receipt, error) {
	receipts := make([]*Receipt, 0)
	var totalUsedGas uint64 = 0
	mGasPool := (*GasPool)(new(big.Int).Set(header.GasLimit))
	for _, tx := range txs {
		// rec, err := ApplyTransaction(stateTree, header, tx, mGasPool, totalUsedGas)
		rec, err := ApplyTransaction(bc, nil, mGasPool, stateTree, header, tx, &totalUsedGas, *bc.GetVMConfig())
		if err != nil {
			txhash := tx.Hash()
			logrus.Errorf("Apply transaction err: hash=%x err=%v", txhash[len(txhash)-4:], err)
			return nil, nil, err
		}
		if rec != nil {
			receipts = append(receipts, rec)
		}
	}
	return new(big.Int).SetUint64(totalUsedGas), receipts, nil
}

func (bc *BlockChain) checkBlockHeaderSanity(prev, header *BlockHeader, blockHash common.Hash) error {
	target := BitsUnzip(header.Bits)
	if target.Sign() <= 0 {
		return fmt.Errorf("bits must be a non-negative integer")
	}
	max := BitsUnzip(bc.genesisBHeader.Bits)
	//target difficuty should be less than the minimum difficuty based on the genesisBlock
	if target.Cmp(max) > 0 {
		return fmt.Errorf("pow check err")
	}
	current := new(big.Int).SetBytes(blockHash[:])
	// the current hash can not be larger than the target hash value
	if current.Cmp(target) > 0 {
		return fmt.Errorf("pow check err")
	}

	if header.Height <= 22180 {
		return nil
	} else if header.Height < uint64(totalblocksv2+totalblocksv3) {
		last, err := bc.calcNextRequiredBitsByHeight(prev.Height)
		if err != nil {
			return err
		}
		if last != header.Bits {
			return fmt.Errorf("pow check err")
		}
	} else if header.Height <= 23555 {
		return nil
	} else if header.Height <= uint64(totalblocks) {
		last, err := bc.calcNextRequiredBitsByHeight(prev.Height)
		if err != nil {
			return err
		}
		if last != header.Bits {
			return fmt.Errorf("pow check err")
		}
	} else {
		return fmt.Errorf("pow check err")
	}

	return nil
}

func (bc *BlockChain) checkTransactionSanity(tx *Transaction) error {
	if !tx.VerifySignature() {
		return fmt.Errorf("VerifySignature err")
	}
	return nil
}

// IntrinsicGas computes the 'intrisic gas' for a message
// with the given data.
func (bc *BlockChain) IntrinsicGas(data []byte) *big.Int {
	return common.CalcTxInitialCost(data)
}

func buyGas(sender *StateObj, tx *Transaction, gp *GasPool, gas *big.Int) error {
	mgval := new(big.Int).Mul(tx.GasPrice, tx.GasLimit)
	if sender.GetBalance().Cmp(mgval) < 0 {
		return fmt.Errorf("per-buy gas err, balance is not enough")
	}
	if err := gp.SubGas(tx.GasLimit); err != nil {
		//logrus.Warnf("gas limit out: %s, gp=%s", tx.GasLimit, gp)
		return err
	}
	gas.Add(gas, tx.GasLimit)
	sender.SubBalance(mgval)
	return nil
}

// func txPreCheck(stateTree *StateTree, tx *Transaction, gp *GasPool, gas *big.Int) (*StateObj, error) {
// 	fromaddr, err := tx.FromAddr()
// 	if err != nil {
// 		return nil, err
// 	}
// 	sender := stateTree.GetOrNewStateObj(fromaddr)
// 	if sender.GetNonce() != tx.Nonce {
// 		return sender, fmt.Errorf("nonce err: want=%d, got=%d", sender.GetNonce(), tx.Nonce)
// 	}
// 	if err = buyGas(sender, tx, gp, gas); err != nil {
// 		return sender, err
// 	}
// 	return sender, nil
// }

func TxToAddrNotSet(tx *Transaction) bool {
	return bytes.Equal(tx.To[:], common.ZeroAddr[:])
}

// func (bc *BlockChain) transfer(st *StateTree, seder *StateObj, to common.Address, amount *big.Int) error {
// 	toObj := st.GetOrNewStateObj(to)
// 	if seder.balance.Cmp(amount) < 0 {
// 		return errors.New("from balance is not enough")
// 	}
// 	seder.SubBalance(amount)
// 	toObj.AddBalance(amount)
// 	return nil
// }

func (bc *BlockChain) GetBlockHashes(from uint64, count uint64) []common.Hash {
	bc.mu.Lock()
	curHeight := bc.currentBHeader.Height
	bc.mu.RUnlock()
	if from+count > curHeight {
		count = curHeight
	}
	hashes := make([]common.Hash, 0)
	for h := uint64(0); from+h <= count; h++ {
		block := bc.GetBlockByNumber(from + h)
		hashes = append(hashes, block.HeaderHash())
	}
	return hashes
}

func (bc *BlockChain) GetBlockHashesFromHash(hash common.Hash, max uint64) (chain []common.Hash) {
	block := bc.GetBlockByHash(hash)
	if block == nil {
		return
	}
	// XXX Could be optimised by using a different database which only holds hashes (i.e., linked list)
	for i := uint64(0); i < max; i++ {
		block = bc.GetBlockByHash(block.HashPrevBlock())
		if block == nil {
			break
		}
		chain = append(chain, block.HeaderHash())
	}

	return
}
func (bc *BlockChain) GetBlocks(from uint64, count uint64) []*Block {
	bc.mu.RLock()
	curheight := bc.currentBHeader.Height
	bc.mu.RUnlock()
	if from+count > curheight {
		count = curheight
	}
	hashes := make([]*Block, 0)
	for h := uint64(0); from+h <= count; h++ {
		block := bc.GetBlockByNumber(from + h)
		if block == nil {
			break
		}
		hashes = append(hashes, block)
	}
	return hashes
}

// FindAncestor tries to locate the common ancestor block of the local chain and
// a remote peers blockchain. In the general case when our node was in sync and
// on the correct chain, checking the top N blocks should already get us a match.
// In the rare scenario when we ended up on a long soft fork (i.e. none of the
// head blocks match), we do a binary search to find the common ancestor.

func (bc *BlockChain) FindAncestor(bHeader *BlockHeader, height uint64) *BlockHeader {
	return bc.findAncestor(bHeader, height)
}

func (bc *BlockChain) findAncestor(bHeader *BlockHeader, height uint64) *BlockHeader {
	if bHeader == nil {
		return nil
	}

	indexFirst := bHeader
	for i := 0; indexFirst != nil && i < int(height); i++ {
		indexFirst = bc.GetBlockHeaderByBHash(indexFirst.HashPrevBlock)
	}
	return indexFirst
}
func (bc *BlockChain) calcNextRequiredBitsByHeight(height uint64) (uint32, error) {
	if height > 1 && GenesisBits == TestNetGenesisBits {
		//logrus.Infof("total: %d, end: %d, pre: %d, height: %d", totalblocks, endTimeV1, targetTimePerBlock, int64(height))
		if int64(height) >= totalblocks {
			return 0, ErrDifficultyOverflow
		}
	}
	lastBlock := bc.getBlockByNumber(height)
	if lastBlock == nil {
		return 0, errors.New("not found block")
	}
	lastHeader := lastBlock.Header
	lastHeight := lastBlock.Height()

	var blocksPerRetarget uint64
	var targetTimespan int64

	if height < uint64(totalblocksv2+totalblocksv3) {
		// V3
		targetTimespan = targetTimespanPreV4
		blocksPerRetarget = uint64(targetTimespan / targetTimePerBlock)
	} else if height < uint64(totalblocksv2+totalblocksv3+totalblocksv4) {
		// V4
		targetTimespan = targetTimespanV4
		blocksPerRetarget = uint64(targetTimespan / targetTimePerBlock)
	}

	// if the height of the next block is not an integral multiple of the target，no changes.
	if (lastHeight+1)%blocksPerRetarget != 0 {
		//logrus.Infof("need ccc, last: %d, blocksPerRetarget: %d", lastHeight+1, blocksPerRetarget)
		return lastHeader.Bits, nil
	}
	first := bc.findAncestor(lastHeader, blocksPerRetarget-1)
	if first == nil {
		//logrus.Infof("need bbb")
		return lastHeader.Bits, nil
	}
	//logrus.Infof("need aaa")
	firstTime := first.Timestamp
	lastTime := lastHeader.Timestamp
	minRetargetTimespan := targetTimespan / adjustmentFactor
	maxRetargetTimespan := targetTimespan * adjustmentFactor
	actualTimespan := int64(lastTime - firstTime)
	adjustedTimespan := actualTimespan
	if actualTimespan < minRetargetTimespan {
		adjustedTimespan = minRetargetTimespan
	} else if actualTimespan > maxRetargetTimespan {
		adjustedTimespan = maxRetargetTimespan
	}
	oldTarget := BitsUnzip(lastHeader.Bits)
	newTarget := new(big.Int).Mul(oldTarget, big.NewInt(adjustedTimespan))
	newTarget.Div(newTarget, big.NewInt(targetTimespan))
	newTarget.Set(common.BigMin(newTarget, BitsUnzip(bc.genesisBHeader.Bits)))
	newTargetBits := BigByZip(newTarget)
	return newTargetBits, nil
}

func (bc *BlockChain) CalcNextRequiredDifficulty() (uint32, error) {
	bc.mu.RLock()
	lastHeader := bc.currentBHeader
	bc.mu.RUnlock()
	return bc.CalcNextRequiredBitsByHeight(lastHeader.Height)
}

func (bc *BlockChain) CalcNextRequiredBitsByHeight(height uint64) (uint32, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.calcNextRequiredBitsByHeight(height)
}

func (bc *BlockChain) CurrentStateTree() *StateTree {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.stateTree
}
