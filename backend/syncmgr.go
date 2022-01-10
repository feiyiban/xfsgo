package backend

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
	"xfsgo"
	"xfsgo/common"
	"xfsgo/p2p"
	"xfsgo/p2p/discover"

	"github.com/sirupsen/logrus"
)

var (
	maxHashesFetch      = uint64(512)
	maxBlocksFetch      = uint64(128)
	timeoutTTL          = 10 * time.Second
	blockFetchTTL       = 10 * time.Second
	errUnKnowPeer       = errors.New("unKnow peer")
	errTimeout          = errors.New("timeout")
	errEmptyHashes      = errors.New("empty hashes")
	errBadHashes        = errors.New("bad hashes")
	errBadPeer          = errors.New("peer is bad")
	errNoPeers          = errors.New("no peers to keep download active")
	errPeersUnavailable = errors.New("no peers available or all peers tried for block download process")
	errCancelHashFetch  = errors.New("hash fetching canceled (requested)")
	errCancelBlockFetch = errors.New("block fetching canceled (requested)")
	errBusy             = errors.New("busy")
	warnunsync          = errors.New("warnunsync")
	//errEmptyHashSet = errors.New("empty hash set by peer")
)

type chainMgr interface {
	CurrentBHeader() *xfsgo.BlockHeader
	GenesisBHeader() *xfsgo.BlockHeader
	GetBlockByNumber(num uint64) *xfsgo.Block
	GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash
	GetBlockByHashWithoutRec(hash common.Hash) *xfsgo.Block
	GetReceiptByHash(hash common.Hash) *xfsgo.Receipt
	GetBlockByHash(hash common.Hash) *xfsgo.Block
	InsertChain(block *xfsgo.Block) error
	SetBoundaries(syncStatsOrigin, syncStatsHeight uint64) error
}
type hashPack struct {
	peerId discover.NodeId
	hashes RemoteHashes
}
type blockPack struct {
	peerId discover.NodeId
	blocks RemoteBlocks
}

type txPack struct {
	peerId discover.NodeId
	txs    RemoteTxs
}
type syncMgr struct {
	version   uint32
	network   uint32
	chain     chainMgr
	eventBus  *xfsgo.EventBus
	peers     *peerSet
	hm        *mHandlerMgr
	txPool    *xfsgo.TxPool
	newPeerCh chan syncpeer
	// chs
	hashPackCh  chan hashPack
	blockPackCh chan blockPack
	txPackCh    chan txPack
	processCh   chan bool
	cancelCh    chan struct{}
	// lock
	//syncLock    sync.Mutex
	processLock   sync.Mutex
	cancelLock    sync.RWMutex
	queue         *syncQueue
	lastReport    time.Time
	synchronising int32
	lastRecord    uint64

	nodeSyncFlag bool
}

func newSyncMgr(
	version, network uint32,
	chain chainMgr,
	eventBus *xfsgo.EventBus,
	txPool *xfsgo.TxPool, nodeSyncFlag bool) *syncMgr {
	mgr := &syncMgr{
		chain:        chain,
		version:      version,
		network:      network,
		nodeSyncFlag: nodeSyncFlag,
		peers:        newPeerSet(),
		eventBus:     eventBus,
		txPool:       txPool,
		newPeerCh:    make(chan syncpeer, 1),
		hashPackCh:   make(chan hashPack, 1),
		blockPackCh:  make(chan blockPack, 1),
		txPackCh:     make(chan txPack, 1),
		processCh:    make(chan bool, 1),
		cancelCh:     make(chan struct{}),
		queue:        newSyncQueue(),
	}
	hm := newHandlerMgr()
	syncHanlder := newSyncHandler(chain, mgr.handleHashes,
		mgr.handleBlocks, mgr.handleNewBlock, mgr.handleTransactions)
	hm.Handle(GetBlockHashesFromNumberMsg, syncHanlder.handleGetBlockHashes)
	hm.Handle(BlockHashesMsg, syncHanlder.handleGotBlockHashes)
	hm.Handle(GetBlocksMsg, syncHanlder.handleGetBlocks)
	hm.Handle(BlocksMsg, syncHanlder.handleGotBlocks)
	hm.Handle(NewBlockMsg, syncHanlder.handleNewBlock)
	hm.Handle(TxMsg, syncHanlder.handleTransactions)
	hm.Handle(GetReceipts, syncHanlder.handleGetReceipts)
	hm.Handle(ReceiptsData, syncHanlder.handleGotReceipts)
	mgr.hm = hm
	return mgr
}

func (mgr *syncMgr) Handler() handlerMgr {
	return mgr.hm
}

func (mgr *syncMgr) onNewPeer(p2ppeer p2p.Peer) error {
	p := newPeer(p2ppeer, mgr.version, mgr.network)
	return mgr.handlePeer(p)
}
func (mgr *syncMgr) handleTransactions(p discover.NodeId, txs RemoteTxs) error {
	pn := mgr.peers.get(p)
	if pn == nil {
		return errUnKnowPeer
	}
	for _, tx := range txs {
		pn.AddTx(tx.Hash)
		var targetTx *xfsgo.Transaction
		_ = common.Objcopy(tx, &targetTx)
		if err := mgr.txPool.Add(targetTx); err != nil {
			//logrus.Warnf("handle transactions msg err: %s", err)
		}
	}
	return nil
}
func (mgr *syncMgr) handleHashes(p discover.NodeId, hashes RemoteHashes) {
	mgr.cancelLock.RLock()
	cancel := mgr.cancelCh
	mgr.cancelLock.RUnlock()
	select {
	case <-cancel:
		return
	default:
	}
	mgr.hashPackCh <- hashPack{
		peerId: p,
		hashes: hashes,
	}
}

func (mgr *syncMgr) handleNewBlock(p discover.NodeId, block *RemoteBlock) error {
	if block == nil {
		return fmt.Errorf("block is null")
	}
	blockHash := block.Header.Hash
	blockHeight := block.Header.Height
	pn := mgr.peers.get(p)
	if pn == nil {
		return errUnKnowPeer
	}
	if blockHeight > pn.Height() {
		mgr.peers.setHeight(p, blockHeight)
		mgr.peers.setHead(p, blockHash)
		go mgr.Synchronise(pn)
	}
	pn.AddBlock(block.Header.Hash)
	go mgr.BroadcastBlock(block)
	return nil
}

func (mgr *syncMgr) handleBlocks(p discover.NodeId, blocks RemoteBlocks) {
	mgr.cancelLock.RLock()
	cancel := mgr.cancelCh
	mgr.cancelLock.RUnlock()
	select {
	case <-cancel:
		return
	default:
	}
	mgr.blockPackCh <- blockPack{
		peerId: p,
		blocks: blocks,
	}
}

func (mgr *syncMgr) handlePeer(p syncpeer) error {
	var err error = nil
	head := mgr.chain.CurrentBHeader()
	genesis := mgr.chain.GenesisBHeader()
	if err = p.Handshake(head.HeaderHash(), head.Height, genesis.HeaderHash()); err != nil {
		return err
	}
	mgr.peers.appendPeer(p)
	mgr.newPeerCh <- p
	defer mgr.peers.dropPeer(p.ID())
	// Send local transaction to remote synchronization
	mgr.syncTransactions(p)
	for {
		if err = mgr.handleMsg(p, p); err != nil {
			return err
		}
	}
}

type sender interface {
	SendData(mType uint8, data []byte) error
	SendObject(mType uint8, data interface{}) error
}
type protocolMsgReader interface {
	ID() discover.NodeId
	GetProtocolMsgCh() (chan p2p.MessageReader, error)
}

func (mgr *syncMgr) handleMsg(s sender, p protocolMsgReader) error {
	msgCh, err := p.GetProtocolMsgCh()
	if err != nil {
		return err
	}
	select {
	case msg := <-msgCh:
		msgCode := msg.Type()
		var data []byte
		data, err = msg.ReadAll()
		if err != nil {
			logrus.Errorf("Handle message err %s", err)
			return err
		}
		if err = mgr.hm.OnMessage(p.ID(), s, msgCode, data); err != nil {
			return err
		}
	default:
	}
	return nil
}

func (mgr *syncMgr) cancel() {
	mgr.cancelLock.Lock()
	if mgr.cancelCh != nil {
		select {
		case <-mgr.cancelCh:
			// Channel was already closed
		default:
			close(mgr.cancelCh)
		}
	}
	mgr.cancelLock.Unlock()
	// Reset the queue
	mgr.queue.Reset()
}

func (mgr *syncMgr) findAncestor(p syncpeer) (uint64, error) {
	//h.fetchAncestorLock.Lock()
	//defer h.fetchAncestorLock.Unlock()
	pid := p.ID()
	var err error = nil
	headBlock := mgr.chain.CurrentBHeader()
	if headBlock == nil {
		return 0, errors.New("empty")
	}
	height := headBlock.Height

	var from = 0
	from = int(height) - int(maxHashesFetch)
	if from < 0 {
		from = 0
	}
	//logrus.Debugf("Find ancestor block hashes: chainHeight=%d, start=%d, count=%d, peerId=%x",
	//	height, from, MaxHashFetch, pid[len(pid)-4:])
	//logrus.Debugf("Find ancestor block hashes: chainHeight=%d, start=%d, count=%d, peerId=%x",
	//	height, from, maxHashesFetch, pid[len(pid)-4:])
	if err = p.RequestHashesFromNumber(uint64(from), maxHashesFetch); err != nil {
		return 0, err
	}
	number := uint64(0)
	haveHash := common.HashZ
	timeout := time.After(timeoutTTL)
	//finished := false
	//loop:
	for finished := false; !finished; {
		select {
		case <-mgr.cancelCh:
			return 0, errCancelHashFetch
		// Skip loop if timeout
		case <-timeout:
			logrus.Warnf("Fetch ancestor hashes timeout: chainHeight=%d, from=%d, count: %d, peerId=%x",
				height, from, maxHashesFetch, pid[len(pid)-4:])
			return 0, errTimeout
		case pack := <-mgr.hashPackCh:
			wanId := p.ID()
			wantPeerId := wanId[:]
			gotPeerId := pack.peerId[:]
			if !bytes.Equal(wantPeerId, gotPeerId) {
				break
			}
			hashes := pack.hashes
			if len(hashes) == 0 {
				logrus.Warnf("Fetch ancestor hashes is emtpy: chainHeight=%d, from=%d, count: %d, peerId=%x",
					height, from, maxHashesFetch, pid[len(pid)-4:])
				return 0, errEmptyHashes
			}
			finished = true
			//logrus.Debugf("Found ancestor hashes: currentHeight=%d, fetchFrom=%d, fetchCount: %d, foundCount=%d, peerId=%x",
			//	height, from, MaxHashFetch, len(hashes), pid[len(pid)-4:])
			for i := len(hashes) - 1; i >= 0; i-- {
				hash := hashes[i]
				//logrus.Debugf("Check ancestor hashes: chainHeight=%d, fetchFrom=%d, fetchCount: %d, foundCount=%d, index=%d, hash=%x, peerId=%x",
				//	height, from, MaxHashFetch, len(hashes), i,hash[len(haveHash)-4:], pid[len(pid)-4:])
				if b := mgr.chain.GetBlockByHash(hash); b != nil {
					number, haveHash = uint64(from)+uint64(i), hashes[i]
					break
				}
			}
		}
	}
	if !bytes.Equal(haveHash[:], common.HashZ[:]) {
		//logrus.Debugf("Found ancestor block: height=%d, hash=%x...%x, peerId=%x...%x",
		//	number, haveHash[:4], haveHash[len(haveHash)-4:], pid[:4], pid[len(pid)-4:])
		return number, nil
	}
	logrus.Warnf("Not found ancestor: currentHeight=%d, from=%d, count=%d, peerId=%x",
		height, from, maxHashesFetch, pid[len(pid)-4:])
	left, right := uint64(0), height
	for left+1 < right {
		//logrus.Debugf("Traversing height range:  left=%d, right=%d", left, right)
		mid := (left + right) / 2
		if err = p.RequestHashesFromNumber(mid, 1); err != nil {
			return 0, err
		}
		timeout := time.After(timeoutTTL)
		for arrived := false; !arrived; {
			select {
			case <-mgr.cancelCh:
				return 0, errCancelHashFetch
			case <-timeout:
				return 0, errTimeout
			case pack := <-mgr.hashPackCh:
				wanId := p.ID()
				wantPeerId := wanId[:]
				gotPeerId := pack.peerId[:]
				if !bytes.Equal(wantPeerId, gotPeerId) {
					break
				}
				hashes := pack.hashes
				if len(hashes) != 1 {
					return 0, errBadHashes
				}
				arrived = true
				if b := mgr.chain.GetBlockByHash(hashes[0]); b != nil {
					left = mid
				} else {
					right = mid
				}
			}
		}
	}
	return left, nil
}

func (mgr *syncMgr) fetchHashes(p syncpeer, from uint64) error {
	pid := p.ID()
	timeout := time.NewTimer(0)
	<-timeout.C
	defer timeout.Stop()
	getHashes := func(num uint64) {
		logrus.Debugf("Fetching Hashes: from=%d, count=%d, peerId=%x", from, maxHashesFetch, pid[len(pid)-4:])
		go func() {
			if err := p.RequestHashesFromNumber(from, maxHashesFetch); err != nil {
				logrus.Warnf("Requst fetch hashes from number err: from=%d, count=%d, err=%s, peerId=%x",
					from, maxHashesFetch, err, pid[len(pid)-4:])
			}
		}()
		timeout.Reset(timeoutTTL)
	}
	getHashes(from)
	//gotHashes := false
	for {
		select {
		case <-mgr.cancelCh:
			return errCancelHashFetch
		case <-timeout.C:
			logrus.Warnf("Fetch hashes timeout: from=%d, count: %d, peerId=%x...%x",
				from, maxHashesFetch, pid[0:4], pid[len(pid)-4:])
			return errTimeout
		case pack := <-mgr.hashPackCh:
			wanId := p.ID()
			wantPeerId := wanId[:]
			gotPeerId := pack.peerId[:]
			if !bytes.Equal(wantPeerId, gotPeerId) {
				break
			}
			timeout.Stop()
			hashes := pack.hashes
			if len(hashes) == 0 {
				//headBlock := h.blockchain.CurrentBHeader()
				//headHeight := headBlock.Height
				select {
				case mgr.processCh <- false:
				}
				//if !gotHashes {
				//	return errBadPeer
				//}
				return nil
			}
			//gotHashes = true
			inserts := mgr.queue.Insert(hashes)
			if len(inserts) != len(pack.hashes) {
				return errBadPeer
			}
			select {
			case mgr.processCh <- true:
			default:
			}
			from += uint64(len(pack.hashes))
			getHashes(from)
		}
	}
}

func (mgr *syncMgr) processQueue() {
	mgr.processLock.Lock()
	defer mgr.processLock.Unlock()
	blocks := mgr.queue.TakeBlocks()
	if len(blocks) == 0 {
		return
	}
	//logrus.Debugf("Inserting chain with %d blocks: start=%d, end=%d",
	//	len(blocks) , blocks[0].rawBlock.Height(), blocks[len(blocks)].rawBlock.Height())
	for len(blocks) != 0 {
		// Retrieve the first batch of blocks to insert
		max := int(math.Min(float64(len(blocks)), float64(maxBlocksFetch)))
		raw := make([]*xfsgo.Block, 0, max)
		for _, block := range blocks[:max] {
			raw = append(raw, block.rawBlock)
		}
		var (
			err       error
			lastBlock *xfsgo.Block
			lastIndex int
		)
		// Try to inset the blocks, drop the originating peer if there's an error
		ignoreBlocks := 0
		orphanBlocks := 0
		for lastIndex, lastBlock = range raw {
			if err = mgr.chain.InsertChain(lastBlock); err != nil {
				switch err {
				case xfsgo.ErrBlockIgnored:
					ignoreBlocks += 1
					err = nil
					continue
				case xfsgo.ErrOrphansBlock:
					orphanBlocks += 1
					err = nil
					continue
				default:
				}
				break
			}
		}
		if err != nil {
			logrus.Errorf("Insert block to chain failed: %v", err)
			mgr.cancel()
			mgr.peers.dropPeer(blocks[lastIndex].originPeer)
			return
		}
		if len(raw) <= 0 {
			return
		}
		start, end := raw[0].Height(), raw[len(raw)-1].Height()
		logrus.Infof("Imported chain with %d blocks: start=%d, end=%d, ignored=%d, orphanBlocks=%d",
			len(raw), start, end, ignoreBlocks, orphanBlocks)
		blocks = blocks[max:]
	}
}

func (mgr *syncMgr) fetchBlocks(from uint64, id discover.NodeId) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	sendFetchRequest := func(p syncpeer, request *fetchBlockRequest) error {
		pid := p.ID()
		requestHashes := request.hashes
		logrus.Debugf("Fetch blocks: count=%d, peerId=%x", len(requestHashes), pid[len(pid)-4:])
		hashes := make([]common.Hash, 0)
		for k := range requestHashes {
			hashes = append(hashes, k)
		}
		return p.RequestBlocks(hashes)
	}
	update := make(chan struct{}, 1)
	mgr.queue.Prepare(from)
	finished := false

	for {
		mgr.report(from, time.Now(), id)
		select {
		case <-mgr.cancelCh:
			return errCancelBlockFetch
		case <-ticker.C:
			select {
			case update <- struct{}{}:
			default:
			}
		case pack := <-mgr.blockPackCh:
			if p := mgr.peers.get(pack.peerId); p != nil {
				blocks := make([]*xfsgo.Block, 0)
				for _, block := range pack.blocks {
					var mBlock *xfsgo.Block
					_ = common.Objcopy(block, &mBlock)
					blocks = append(blocks, mBlock)
				}
				err := mgr.queue.Deliver(p, blocks)
				if err != nil {
					logrus.Errorf("Fetch block err: %v", err)
				}
				go mgr.processQueue()
			}
			select {
			case update <- struct{}{}:
			default:
			}
		case fetchHashes := <-mgr.processCh:
			if !fetchHashes {
				finished = true
			}
			select {
			case update <- struct{}{}:
			default:
			}
		case <-update:
			if mgr.peers.empty() {
				return errNoPeers
			}
			for _, pid := range mgr.queue.Expire(blockFetchTTL) {
				if p := mgr.peers.get(pid); p != nil {
					// TODO: down
					logrus.Warnf("block delivery timeout: %x", pid[len(pid)-4:])
				}
			}
			if mgr.queue.Pending() == 0 {
				if mgr.queue.FetchPending() == 0 && finished {
					return nil
				}
				break
			}
			for _, p := range mgr.peers.listAndShort() {
				if mgr.queue.Throttle() {
					break
				}
				request := mgr.queue.Reserve(p, 20)
				if request == nil {
					continue
				}
				if err := sendFetchRequest(p, request); err != nil {
					mgr.queue.Cancel(request)
				}
			}
			if !mgr.queue.Throttle() && mgr.queue.FetchPending() == 0 {
				return errPeersUnavailable
			}
		}
	}
}
func (mgr *syncMgr) recordSync(num uint64) {
	if num > mgr.lastRecord {
		mgr.lastRecord = num
	}
}
func (mgr *syncMgr) report(v uint64, t time.Time, id discover.NodeId) {
	if t.Sub(mgr.lastReport) < (1 * time.Second) {
		return
	}
	head := mgr.chain.CurrentBHeader()
	nowHeight := head.Height
	if nowHeight-(v-1) > 0 && nowHeight < mgr.lastRecord {
		total := mgr.lastRecord - v
		completed := nowHeight - (v - 1)
		progress := float64(completed) / float64(total) * float64(100)
		logrus.Infof("Sync in progress: synced=%.2f%%, id=%x", progress, id[len(id)-4:])
	}
	mgr.lastReport = t
}
func (mgr *syncMgr) syncWithPeer(p syncpeer) error {
	if p == nil {
		return nil
	}
	var (
		pId = p.ID()
		err error
	)
	mgr.eventBus.Publish(xfsgo.SyncStartEvent{})
	defer func() {
		if err != nil {
			mgr.cancel()
			mgr.eventBus.Publish(xfsgo.SyncFailedEvent{Error: err})
		} else {
			mgr.eventBus.Publish(xfsgo.SyncDoneEvent{})
		}
	}()
	logrus.Debugf("Synchronise from peer: id=%x", pId[len(pId)-4:])

	var number uint64
	if number, err = mgr.findAncestor(p); err != nil {
		return err
	}
	_ = mgr.chain.SetBoundaries(number, p.Height())
	mgr.recordSync(p.Height())

	logrus.Debugf("Successfully find ancestor: number=%d, peerId=%x", number, pId[len(pId)-4:])
	errc := make(chan error, 2)
	go func() {
		errc <- mgr.fetchHashes(p, number+1)
	}()
	go func() {
		errc <- mgr.fetchBlocks(number+1, pId)
	}()
	if err = <-errc; err != nil {
		mgr.cancel()
		<-errc
		return err
	}
	return <-errc
}

func (mgr *syncMgr) synchronise(pid discover.NodeId) error {
	if !atomic.CompareAndSwapInt32(&mgr.synchronising, 0, 1) {
		return errBusy
	}
	defer atomic.StoreInt32(&mgr.synchronising, 0)
	mgr.queue.Reset()
	mgr.peers.reset()
	mgr.cancelLock.Lock()
	mgr.cancelCh = make(chan struct{})
	mgr.cancelLock.Unlock()
	ps := mgr.peers
	var p syncpeer
	if p = ps.get(pid); p == nil {
		return errUnKnowPeer
	}

	if !mgr.nodeSyncFlag {
		return warnunsync
	}

	return mgr.syncWithPeer(p)
}

func (mgr *syncMgr) Synchronise(p syncpeer) {
	if p == nil {
		return
	}
	chainHead := mgr.chain.CurrentBHeader()
	currentHeight := chainHead.Height
	//logrus.Infof("chainHead: %d, pheight: %d", currentHeight, p.Height())
	if p.Height() <= currentHeight {
		return
	}
	switch err := mgr.synchronise(p.ID()); err {
	case nil:
		logrus.Infof("Synchronisation completed")
	case errBusy:
	case warnunsync:
	default:
		logrus.Errorf("Synchronisation failed: %v", err)
	}
}

func (mgr *syncMgr) syncer() {
	forceSync := time.NewTicker(10 * time.Second)
	defer forceSync.Stop()
	for {
		select {
		case <-mgr.newPeerCh:
			if mgr.peers.count() < 5 {
				break
			}
			go mgr.Synchronise(mgr.peers.basePeer())
		case <-forceSync.C:
			go mgr.Synchronise(mgr.peers.basePeer())
		}
	}
}
func (mgr *syncMgr) syncTransactions(p syncpeer) {
	if mgr.txPool == nil {
		return
	}
	txs := mgr.txPool.GetTransactions()
	if len(txs) == 0 {
		return
	}
	mTxs := coverTxs2RemoteBlockTxs(txs)
	mgr.txPackCh <- txPack{
		peerId: p.ID(),
		txs:    mTxs,
	}
}
func (mgr *syncMgr) txSyncLoop() {
	send := func(pack txPack) {
		peerId := pack.peerId
		if p := mgr.peers.get(peerId); p != nil {
			if err := p.SendTransactions(pack.txs); err != nil {
				logrus.Warnf("send txs err: %s", err)
			}
		}
	}
	for {
		select {
		case pack := <-mgr.txPackCh:
			if !mgr.nodeSyncFlag {
				continue
			}
			send(pack)
		}
	}
}

func (mgr *syncMgr) BroadcastBlock(block *RemoteBlock) {
	if !mgr.nodeSyncFlag {
		return
	}
	for _, p := range mgr.peers.peerList() {
		if p.HasBlock(block.Header.Hash) {
			continue
		}
		if err := p.SendNewBlock(block); err != nil {
			continue
		}
	}
}

func (mgr *syncMgr) txBroadcastLoop() {

	txPreEventSub := mgr.eventBus.Subscript(xfsgo.TxPreEvent{})
	defer txPreEventSub.Unsubscribe()
	for {
		select {
		case e := <-txPreEventSub.Chan():
			event := e.(xfsgo.TxPreEvent)
			tx := event.Tx
			rmtx := coverTx2RemoteTx(tx)
			mgr.BroadcastTx(rmtx)
		}
	}
}
func (mgr *syncMgr) BroadcastTx(tx *RemoteBlockTx) {
	mHeader := mgr.chain.CurrentBHeader()
	mHeight := mHeader.Height
	if !mgr.nodeSyncFlag {
		return
	}

	for _, p := range mgr.peers.peerList() {
		if p.Height() < mHeight {
			continue
		}
		if p.HasTx(tx.Hash) {
			continue
		}
		if err := p.SendTransactions(RemoteTxs{tx}); err != nil {
			continue
		}
	}
}
func (mgr *syncMgr) minedBroadcastLoop() {
	newMinerBlockEventSub := mgr.eventBus.Subscript(xfsgo.NewMinedBlockEvent{})
	defer newMinerBlockEventSub.Unsubscribe()
	for {
		select {
		case e := <-newMinerBlockEventSub.Chan():
			event := e.(xfsgo.NewMinedBlockEvent)
			block := event.Block
			rmblk := coverBlock2RemoteBlock(block)
			go mgr.BroadcastBlock(rmblk)
		}
	}
}
func (mgr *syncMgr) Start() {
	// start broadcasing transaction
	go mgr.txBroadcastLoop()
	// start broadcasing block
	go mgr.minedBroadcastLoop()
	// start syncmgrronising block
	go mgr.syncer()
	// start syncmgrronising transaction
	go mgr.txSyncLoop()
}
