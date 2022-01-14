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

package miner

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"xfsgo"
	"xfsgo/common"
	"xfsgo/state"
	"xfsgo/storage/badger"

	"github.com/sirupsen/logrus"
)

const (
	// hpsUpdateSecs is the number of seconds to wait in between each
	// update to the hashes per second monitor.
	hashUpdateSecs = 1

	// hashUpdateSec is the number of seconds each worker waits in between
	// notifying the speed monitor with how many hashes have been completed
	// while they are actively searching for a solution.  This is done to
	// reduce the amount of syncs between the workers that must be done to
	// keep track of the hashes per second.
	hpsUpdateSecs = 10

	// maxNonce is the maximum value a nonce can be in a block header.
	maxNonce = ^uint32(0) // 2^32 - 1

	// maxExtraNonce is the maximum value an extra nonce used in a coinbase
	// transaction can be.
	maxExtraNonce = ^uint64(0) // 2^64 - 1
)

var (
	hashRateLoopIntervalSec = 1
	progressReportTime      = 10 * time.Second
	maxWorkers              = uint32(255)
	defaultNumWorkers       = uint32(runtime.NumCPU())
	applyTransactionsErr    = errors.New("apply transaction err")
)

type Config struct {
	Coinbase   common.Address
	Numworkers uint32
}

// Miner creates blocks with transactions in tx pool and searches for proof-of-work values.
type Miner struct {
	*Config
	mu      sync.Mutex
	rwmu    sync.RWMutex
	started bool
	quit    chan struct{}
	// runningWorkers   []chan struct{}
	updateNumWorkers chan uint32
	numWorkers       uint32
	eventBus         *xfsgo.EventBus
	canStart         bool
	shouldStart      bool
	pool             *xfsgo.TxPool
	chain            xfsgo.IBlockChain
	stateDb          badger.IStorage
	gasPrice         *big.Int
	gasLimit         *big.Int
	LastStartTime    time.Time
	rmlock           sync.RWMutex
	remove           map[common.Hash]*xfsgo.Transaction
	wg               sync.WaitGroup
	workerWg         sync.WaitGroup
	runningHashRate  chan common.HashRate
	lastHashRate     common.HashRate
	reportHashes     chan uint64
}

func NewMiner(config *Config,
	stateDb badger.IStorage, chain xfsgo.IBlockChain, eventBus *xfsgo.EventBus, pool *xfsgo.TxPool,
	gasPrice, gasLimit *big.Int) *Miner {
	m := &Miner{
		Config:           config,
		chain:            chain,
		stateDb:          stateDb,
		numWorkers:       0,
		updateNumWorkers: make(chan uint32),
		pool:             pool,
		canStart:         true,
		shouldStart:      false,
		started:          false,
		eventBus:         eventBus,
		gasLimit:         gasLimit,
		gasPrice:         gasPrice,
		remove:           make(map[common.Hash]*xfsgo.Transaction),
		reportHashes:     make(chan uint64, 1),
		runningHashRate:  make(chan common.HashRate),
	}
	go m.mainLoop()
	return m
}

func (m *Miner) GetGasPrice() *big.Int {
	return m.gasPrice
}

func (m *Miner) GetGasLimit() *big.Int {
	m.rwmu.RLock()
	defer m.rwmu.RUnlock()
	return m.gasLimit
}

func (m *Miner) GetWorkerNum() uint32 {
	return m.numWorkers
}

func (m *Miner) GetMinStatus() bool {
	return m.started
}
func (m *Miner) GetNext() bool {
	return m.started
}

func (m *Miner) RunningHashRate() common.HashRate {
	select {
	case r := <-m.runningHashRate:
		return r
	default:
	}
	return m.lastHashRate
}
func (m *Miner) SetGasLimit(limit *big.Int) error {
	m.rwmu.Lock()
	defer m.rwmu.Unlock()
	if limit.Cmp(common.MinGasLimit) < 0 {
		return errors.New("gas limit too low")
	} else if limit.Cmp(common.GenesisGasLimit) > 0 {
		return errors.New("gas limit out of GenesisGasLimit")
	} else if limit.Cmp(m.gasLimit) == 0 {
		return nil
	}
	m.gasLimit = limit
	return nil
}
func (m *Miner) SetGasPrice(price *big.Int) error {
	if price.Cmp(common.Big1) < 0 {
		return errors.New("gas price too low")
	} else if price.Cmp(m.gasPrice) == 0 {
		return nil
	}
	gasPriceChanged := &xfsgo.GasPriceChanged{
		Price: price,
	}
	m.gasPrice = price
	m.eventBus.Publish(gasPriceChanged)
	return nil
}

// Start starts up xfs mining
func (m *Miner) Start(w uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started || !m.canStart {
		return
	}
	workers := w
	if workers == 0 && m.Config.Numworkers != 0 {
		workers = m.Config.Numworkers
	} else if workers == 0 {
		workers = defaultNumWorkers
	}

	m.quit = make(chan struct{})
	m.wg.Add(1)
	go m.miningWorkerController(workers)
	m.LastStartTime = time.Now()
	m.numWorkers = workers
	m.started = true
	m.shouldStart = true
}
func (m *Miner) SetWorkers(num uint32) error {
	if num < 1 {
		return errors.New("number too low")
	} else if num > maxWorkers {
		return errors.New("number over max value")
	}
	m.updateNumWorkers <- num
	m.numWorkers = num
	return nil
}
func (m *Miner) SetCoinbase(address common.Address) {
	m.Coinbase = address
}

// mainLoop is the miner's main event loop, waiting for and reacting to synchronize events.
func (m *Miner) mainLoop() {
	startSub := m.eventBus.Subscript(xfsgo.SyncStartEvent{})
	doneSub := m.eventBus.Subscript(xfsgo.SyncDoneEvent{})
	failedSub := m.eventBus.Subscript(xfsgo.SyncFailedEvent{})
	defer func() {
		startSub.Unsubscribe()
		doneSub.Unsubscribe()
		failedSub.Unsubscribe()
	}()
out:
	for {
		select {
		case <-startSub.Chan():
			m.mu.Lock()
			m.canStart = false
			m.mu.Unlock()
			m.Stop()
		case <-failedSub.Chan():
			m.mu.Lock()
			m.canStart = true
			m.mu.Unlock()
			if m.shouldStart {
				m.Start(m.numWorkers)
			}
		case <-doneSub.Chan():
			m.mu.Lock()
			m.canStart = true
			m.mu.Unlock()

			if m.shouldStart {
				m.Start(m.numWorkers)
			}
			break out
		}
	}
}

func (m *Miner) appendRemove(tx *xfsgo.Transaction) {
	m.rmlock.Lock()
	defer m.rmlock.Unlock()
	txhash := tx.Hash()
	if _, exists := m.remove[txhash]; exists {
		return
	}
	m.remove[txhash] = tx
}
func (m *Miner) doRemove() {
	m.rmlock.RLock()
	defer m.rmlock.RUnlock()
	txs := make([]*xfsgo.Transaction, 0)
	for _, tx := range m.remove {
		txs = append(txs, tx)
		delete(m.remove, tx.Hash())
	}
	if len(txs) != 0 {
		m.pool.RemoveTransactions(txs)
	}
}

func (m *Miner) TargetHeight() uint64 {
	return m.chain.CurrentBHeader().Height + 1
}

func (m *Miner) NextDifficulty() float64 {
	bits, _ := m.chain.CalcNextRequiredDifficulty()
	return xfsgo.CalcDifficultyByBits(bits)
}

func (m *Miner) TargetHashRate() common.HashRate {
	bits, _ := m.chain.CalcNextRequiredDifficulty()
	return xfsgo.CalcHashRateByBits(bits)
}

func (m *Miner) applyTransactions(
	stateTree *state.StateDB,
	header *xfsgo.BlockHeader,
	txs []*xfsgo.Transaction,
	ignoreTxs map[common.Address]struct{},
	commitTxs *[]*xfsgo.Transaction) (*big.Int, []*xfsgo.Receipt, error) {
	receipts := make([]*xfsgo.Receipt, 0)
	var totalUsedGas uint64 = 0
	mGasPool := (*xfsgo.GasPool)(new(big.Int).Set(header.GasLimit))
	//pergp := (*big.Int)(mGasPool)
	//logrus.Debugf("Tx gas limit out of block limit-init: hash=%x, from=%x, mGasPool=%s", txfrom, pergp)
	for _, tx := range txs {
		txfrom, _ := tx.FromAddr()
		txhash := tx.Hash()
		_ = txhash
		if _, exists := ignoreTxs[txfrom]; exists {
			//logrus.Warnf("Tx exists ignore obj: hash=%x, from=%x",
			//	txhash[len(txhash)-4:], txfrom)
			continue
		}
		rec, err := xfsgo.ApplyTransaction(m.chain, nil, mGasPool, stateTree, header, tx, &totalUsedGas, *m.chain.GetVMConfig())
		// rec, err := m.chain.ApplyTransaction(stateTree, header, tx, mGasPool, totalUsedGas)
		if err != nil {
			if err.Error() == xfsgo.GasPoolOutErr.Error() {
				//logrus.Errorf("Miner apply transaction err will be ignore: %s", err)
				ignoreTxs[txfrom] = struct{}{}
				continue
			}
			logrus.Warnf("Miner apply transaction err will be remove: %s", err)
			m.appendRemove(tx)
			return nil, nil, err
		}
		if rec != nil {
			receipts = append(receipts, rec)
		}
		//logrus.Debugf("Commit tx: %x",txhash[len(txhash)-4:])
		*commitTxs = append(*commitTxs, tx)
	}
	return new(big.Int).SetUint64(totalUsedGas), receipts, nil
}

func (m *Miner) mimeBlockWithParent(
	stateTree *state.StateDB,
	parentBlock *xfsgo.BlockHeader,
	coinbase common.Address,
	txs []*xfsgo.Transaction,
	quit chan struct{},
	ticker *time.Ticker,
	fn reportFn) (*xfsgo.Block, error) {
	if parentBlock == nil {
		return nil, errors.New("parentBlock is nil")
	}
	//create a Blockheader which will be the header of the new block.
	lastGenerated := time.Now().Unix()
	header := &xfsgo.BlockHeader{
		Height:        parentBlock.Height + 1,
		HashPrevBlock: parentBlock.HeaderHash(),
		Timestamp:     uint64(lastGenerated),
		Coinbase:      coinbase,
	}
	header.GasUsed = new(big.Int)

	header.GasLimit = common.TxPoolGasLimit
	//calculate the next difficuty for hash value of next block.
	var err error
	header.Bits, err = m.chain.CalcNextRequiredDifficulty()
	if err != nil {
		return nil, err
	}
	committx := make([]*xfsgo.Transaction, 0)
	ignoretxs := make(map[common.Address]struct{})
	gasused, res, err := m.applyTransactions(
		stateTree, header, txs, ignoretxs, &committx)
	if err != nil {
		return nil, applyTransactionsErr
	}
	header.GasUsed = gasused
	xfsgo.AccumulateRewards(stateTree, header)
	stateTree.UpdateAll()
	stateRootBytes := stateTree.Root()
	stateRootHash := common.Bytes2Hash(stateRootBytes)
	header.StateRoot = stateRootHash
	//create a new block and execite the consensus algorithms
	perBlock := xfsgo.NewBlock(header, committx, res)
	return m.execPow(parentBlock, perBlock, quit, ticker, fn)
}

// run the consensus algorithms
func (m *Miner) execPow(last *xfsgo.BlockHeader, perBlock *xfsgo.Block, quit chan struct{}, ticker *time.Ticker, report reportFn) (*xfsgo.Block, error) {
	targetDifficulty := xfsgo.BitsUnzip(perBlock.Bits())
	target := targetDifficulty.Bytes()
	targetHash := make([]byte, 32)
	copy(targetHash[32-len(target):], target)
	hashesCompleted := uint64(0)

	enOffset, err := common.RandomUint64()
	if err != nil {
		logrus.Errorf("Unexpected error while generating random "+
			"extra nonce offset: %v", err)
		enOffset = 0
	}
	for extraNonce := uint64(0); extraNonce < maxExtraNonce; extraNonce++ {
		perBlock.UpdateExtraNonce(extraNonce + enOffset)
		// logrus.Debugf("extraNonce:%v,enOffset:%v, Sum:%v", extraNonce, enOffset, perBlock.ExtraNonce())
		for nonce := uint32(0); nonce <= maxNonce; nonce++ {
			select {
			case <-quit:
				return nil, fmt.Errorf("no block")
			case <-ticker.C:
				select {
				case m.reportHashes <- hashesCompleted:
					hashesCompleted = 0
				default:
				}
				lashBlock := m.chain.CurrentBHeader()
				lastHeight := lashBlock.Height
				currentBlockHeight := perBlock.Height()
				//exit this loop if the current height is updated and larger than the height of the blockchain.
				if lastHeight >= currentBlockHeight {
					//logrus.Debugf("current height of blockchain has been updated: %d, current height: %d", lastHeight, currentBlockHeight)
					return nil, fmt.Errorf("no block")
				}
			default:
				// Non-blocking select to fall through
			}
			report(time.Now(), last)
			perBlock.UpdateNonce(nonce)
			hash := perBlock.HeaderHash()
			hashesCompleted += 2
			if bytes.Compare(hash.Bytes(), targetHash) <= 0 {
				lashBlock := m.chain.CurrentBHeader()
				lastHeight := lashBlock.Height
				currentBlockHeight := perBlock.Height()
				select {
				case m.reportHashes <- hashesCompleted:
				default:
				}
				if lastHeight >= currentBlockHeight {
					return nil, fmt.Errorf("no block")
				}
				return perBlock, nil
			}
		}
	}

	return nil, fmt.Errorf("no block")

}

type reportFn func(now time.Time, lastblock *xfsgo.BlockHeader)

func (m *Miner) generateBlocks(num uint32, quit chan struct{}, report reportFn) {
	ticker := time.NewTicker(time.Second * hashUpdateSecs)
	defer ticker.Stop()

out:
	for {
		select {
		case <-quit:
			break out
		default:
		}
		txs := m.pool.GetTransactions()
		//js,_ :=  json.Marshal(txs)
		//logrus.Debugf("txs(un-sort): %s", js)
		xfsgo.SortByPriceAndNonce(txs)
		lastBlock := m.chain.CurrentBHeader()
		lastStateRoot := lastBlock.StateRoot
		//lastBlockHash := lastBlock.Hash()
		//logrus.Debugf("Generating block by parent height=%d, hash=0x%x...%x, workerId=%-3d", lastBlock.Height(), lastBlockHash[:4], lastBlockHash[len(lastBlockHash)-4:], num)
		stateDB := state.NewStateDB(m.stateDb, lastStateRoot.Bytes())
		startTime := time.Now()
		block, err := m.mimeBlockWithParent(stateDB, lastBlock, m.Coinbase, txs, quit, ticker, report)
		if err != nil {
			switch err {
			case applyTransactionsErr:
				m.doRemove()
			case xfsgo.ErrDifficultyOverflow:
				return
			default:
				continue
			}
		}
		if block == nil {
			continue out
		}
		timeused := time.Now().Sub(startTime)

		hash := block.HeaderHash()
		workload := xfsgo.CalcWorkloadByBits(block.Bits())
		workloadUint64 := workload.Uint64()
		rate := float64(workloadUint64) / timeused.Seconds()
		hashrate := common.HashRate(rate)
		logrus.Infof("Sussessfully sealed new block: height=%d, hash=0x%x, txcount=%d, used=%fs, rate=%s",
			block.Height(), hash[len(hash)-4:], len(block.Transactions), timeused.Seconds(), hashrate)
		if err = stateDB.Commit(); err != nil {
			logrus.Warnln("State tree commit err: ", err)
			continue out
		}
		if err = m.chain.WriteBlock(block); err != nil {
			logrus.Warnln("Write block err: ", err)
			continue out
		}
		//sr := block.StateRoot()
		//logrus.Debugf("successfully Write new block, height=%d, hash=0x%x, workerId=%-3d", block.Height(), hash[len(hash)-4:], num)
		//st := xfsgo.NewStateTree(m.stateDb, sr.Bytes())
		//balance := st.GetBalance(m.Coinbase)
		//logrus.Infof("current coinbase: %s, balance: %d", m.Coinbase.B58String(), balance)
		m.eventBus.Publish(xfsgo.NewMinedBlockEvent{Block: block})
	}
	m.workerWg.Done()
}
func closeWorkers(cs []chan struct{}) {
	for _, c := range cs {
		close(c)
	}
}

func (m *Miner) hashRateLoop(close chan struct{}) {
	//var hashesPerSec float64
	var (
		totalHashes  uint64
		hashesPerSec float64
	)
	intervalNumber := time.Duration(hashRateLoopIntervalSec)
	ticker := time.NewTicker(time.Second * intervalNumber)
	defer ticker.Stop()
out:
	for {
		select {
		// Periodic updates from the workers with how many hashes they
		// have performed.
		case numHashes := <-m.reportHashes:
			totalHashes += numHashes
		// Time to update the hashes per second.
		case <-ticker.C:
			hashPerSec := float64(totalHashes) / float64(hashRateLoopIntervalSec)
			if hashesPerSec == 0 {
				hashesPerSec = hashPerSec
			}
			hashesPerSec = (hashesPerSec + hashPerSec) / 2
			totalHashes = 0
			m.lastHashRate = common.HashRate(hashesPerSec)
		case m.runningHashRate <- common.HashRate(hashesPerSec):
		case <-close:
			break out
		}
	}
}
func (m *Miner) miningWorkerController(worker uint32) {
	var runningWorkers []chan struct{}
	hashrateloopchan := make(chan struct{})
	defer close(hashrateloopchan)
	go m.hashRateLoop(hashrateloopchan)

	var reporting int32
	var lastReportTime time.Time
	report := func(now time.Time, lastblock *xfsgo.BlockHeader) {
		if !atomic.CompareAndSwapInt32(&reporting, 0, 1) {
			return
		}
		defer atomic.StoreInt32(&reporting, 0)
		if now.Sub(lastReportTime) < (progressReportTime) {
			return
		}
		targetHeight := lastblock.Height + 1
		bits, _ := m.chain.CalcNextRequiredBitsByHeight(lastblock.Height)
		workLoad := xfsgo.CalcWorkloadByBits(bits)
		hashRate := m.RunningHashRate()
		hashRateInt := new(big.Int).SetInt64(int64(hashRate))
		estimateTimeStr := "long"
		if hashRateInt.Sign() > 0 {
			estimateTime := new(big.Int).Div(workLoad, hashRateInt)
			estimateTimeStr = fmt.Sprintf("%ds", estimateTime)
		}
		logrus.Infof("Generating new block: targetHeight=%d, targetBits=%d, hashRate=%s, estimate=%s, works=%d",
			targetHeight, bits, hashRate, estimateTimeStr, len(runningWorkers))
		lastReportTime = now
	}
	launchWorkers := func(numWorkers uint32) {
		logrus.Infof("Launch workers count=%d", numWorkers)

		for i := uint32(0); i < numWorkers; i++ {
			quit := make(chan struct{})
			runningWorkers = append(runningWorkers, quit)
			//logrus.Debugf("Start-up woker id=%-3d", i)
			m.workerWg.Add(1)
			go m.generateBlocks(i, quit, report)
		}
	}
	runningWorkers = make([]chan struct{}, 0)
	launchWorkers(worker)
	txPreEventSub := m.eventBus.Subscript(xfsgo.TxPreEvent{})
	defer txPreEventSub.Unsubscribe()
out:
	for {
		select {
		case <-m.quit:
			closeWorkers(runningWorkers)
			m.reset()
			break out
		case e := <-txPreEventSub.Chan():
			event := e.(xfsgo.TxPreEvent)
			Tx := event.Tx
			_ = m.pool.Add(Tx)
		case targetNum := <-m.updateNumWorkers:
			numRunning := uint32(len(runningWorkers))
			if targetNum == numRunning {
				continue
			}
			logrus.Debugf("Update worker: targetNum=%d, currentNum=%d", targetNum, numRunning)
			if targetNum > numRunning {
				launchWorkers(targetNum - numRunning)
				continue
			}
			for i := numRunning - 1; i >= targetNum; i-- {
				close(runningWorkers[i])
				runningWorkers[i] = nil
				runningWorkers = runningWorkers[:i]
			}
			logrus.Infof("Success update worker: targetNum=%d, runningWorkers=%d", targetNum, len(runningWorkers))
		}
	}

	m.workerWg.Wait()
	m.wg.Done()
	logrus.Info("Miner quit")
}

func (m *Miner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started {
		close(m.quit)
		m.wg.Wait()
	}
}
func (m *Miner) reset() {
	//m.mu.Lock()
	//defer m.mu.Unlock()
	m.started = false
}
