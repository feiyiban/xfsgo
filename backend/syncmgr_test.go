package backend

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"
	"xfsgo"
	"xfsgo/common"
	"xfsgo/crypto"
	"xfsgo/p2p"
	"xfsgo/p2p/discover"
	"xfsgo/test"
)

type syncMgrTester struct {
	t   *testing.T
	mgr *syncMgr
}
type testNode struct {
	key    *ecdsa.PrivateKey
	nodeId discover.NodeId
}

func genRandomTestNode() *testNode {
	key := crypto.MustGenPrvKey()
	node := &testNode{
		key: key,
	}
	node.nodeId = discover.PubKey2NodeId(key.PublicKey)
	return node
}

var (
	errEOF         = errors.New("EOF")
	testVersion    = uint32(1)
	testNetwork    = uint32(1)
	testGasPrice   = common.Big10
	testEventBus   = xfsgo.NewEventBus()
	testMemStorage = test.NewMemStorage()
	testGenesis    = xfsgo.NewBlock(&xfsgo.BlockHeader{
		Version:       0,
		Height:        0,
		HashPrevBlock: common.Hash{},
		Timestamp:     0,
		Coinbase:      common.Address{},
		GasLimit:      new(big.Int).SetInt64(0),
		GasUsed:       new(big.Int).SetInt64(0),
		Bits:          0,
		Nonce:         0,
		ExtraNonce:    0,
	}, nil, nil)
	testNodes = []*testNode{
		genRandomTestNode(),
		genRandomTestNode(),
		genRandomTestNode(),
		genRandomTestNode(),
		genRandomTestNode(),
		genRandomTestNode(),
	}
	testSendTTL = 1 * time.Second
)

type checkFn func(t uint8, data []byte) error
type resultCheckSender struct {
	checkFn checkFn
}

func newResultCheckSender(fn checkFn) *resultCheckSender {
	return &resultCheckSender{
		checkFn: fn,
	}
}

func (o *resultCheckSender) SendObject(t uint8, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return o.SendData(t, data)
}
func (o *resultCheckSender) SendData(t uint8, data []byte) error {
	return o.checkFn(t, data)
}

type msgOnceSendTester struct {
	id       discover.NodeId
	ch       chan p2p.MessageReader
	close    chan struct{}
	duration time.Duration
}

func newMsgOnceSendTester(id discover.NodeId, duration time.Duration) *msgOnceSendTester {
	return &msgOnceSendTester{
		ch:       make(chan p2p.MessageReader),
		close:    make(chan struct{}),
		duration: duration,
		id:       id,
	}
}
func (o *msgOnceSendTester) ID() discover.NodeId {
	return o.id
}
func (o *msgOnceSendTester) SendObject(t uint8, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return o.SendData(t, data)
}
func (o *msgOnceSendTester) SendData(t uint8, data []byte) error {
	defer close(o.close)
	time.Sleep(o.duration)
	cpy := &testMessageReader{
		raw:   nil,
		mType: t,
		data:  bytes.NewReader(data),
	}

	o.ch <- cpy
	return nil
}
func (o *msgOnceSendTester) GetProtocolMsgCh() (chan p2p.MessageReader, error) {
	select {
	case <-o.close:
		return nil, errEOF
	default:
	}
	return o.ch, nil
}

func newTestTxPool(chain chainMgr, gasLimit, gasPrice *big.Int) *xfsgo.TxPool {
	statefn := func() *xfsgo.StateTree {
		chainhead := chain.CurrentBHeader()
		return xfsgo.NewStateTree(testMemStorage, chainhead.StateRoot.Bytes())
	}
	gasLimitFn := func() *big.Int {
		return gasLimit
	}
	return xfsgo.NewTxPool(statefn, gasLimitFn, gasPrice, testEventBus)
}
func newSyncMgrTester(t *testing.T, chain chainMgr, txPool *xfsgo.TxPool) *syncMgrTester {
	mgr := newSyncMgr(testVersion, testNetwork, chain, testEventBus, txPool, false)
	return &syncMgrTester{
		t:   t,
		mgr: mgr,
	}
}

func (s *syncMgrTester) handleMsg(sender sender, reader protocolMsgReader) error {
	errs := make(chan error)
	go func() {
		for {
			errs <- s.mgr.handleMsg(sender, reader)
		}
	}()
	for {
		select {
		case e := <-errs:
			if e != nil {
				return e
			}
		default:
		}
	}
}

func TestHandleMsg_handleGetBlockHashes(t *testing.T) {
	chain := newTestChainMgr(testGenesis, common.Address{})
	txpool := newTestTxPool(chain, chain.genesis.Header.GasLimit, testGasPrice)
	mgr := newSyncMgrTester(t, chain, txpool)
	send := newResultCheckSender(func(tt uint8, data []byte) error {
		if tt != BlockHashesMsg {
			return fmt.Errorf("check type err: want=%d, got=%d", BlockHashesMsg, tt)
		}
		t.Logf("result data: %s", string(data))
		var resultJsonObj RemoteHashes
		err := json.Unmarshal(data, &resultJsonObj)
		if err != nil {
			return err
		}
		return nil
	})
	reader := newMsgOnceSendTester(testNodes[0].nodeId, testSendTTL)
	go func() {
		req := &struct {
			From  uint64 `json:"from"`
			Count uint64 `json:"count"`
		}{
			From:  900,
			Count: 0,
		}
		_ = reader.SendObject(GetBlockHashesFromNumberMsg, req)
	}()
	err := mgr.handleMsg(send, reader)
	if err != nil && err != errEOF {
		t.Fatal(err)
	}
}

func TestHandleMsg_handleGotBlockHashes(t *testing.T) {
	chain := newTestChainMgr(testGenesis, common.Address{})
	txpool := newTestTxPool(chain, chain.genesis.Header.GasLimit, testGasPrice)
	mgr := newSyncMgrTester(t, chain, txpool)
	reader := newMsgOnceSendTester(testNodes[0].nodeId, testSendTTL)
	go func() {
		req := make(RemoteHashes, 0)
		req = append(req, chain.genesis.HeaderHash())
		_ = reader.SendObject(BlockHashesMsg, req)
	}()
	err := mgr.handleMsg(reader, reader)
	if err != nil && err != errEOF {
		t.Fatal(err)
	}
}

func TestHandleMsg_handleGetBlocks(t *testing.T) {
	chain := newTestChainMgr(testGenesis, common.Address{})
	maxChain := chain.Copy()
	maxChain.NewEmptyBlocks(100)
	wantHashes := maxChain.GetBlockHashesFromHash(maxChain.last.HeaderHash(), 99)
	txpool := newTestTxPool(maxChain, maxChain.genesis.Header.GasLimit, testGasPrice)
	mgr := newSyncMgrTester(t, maxChain, txpool)
	send := newResultCheckSender(func(tt uint8, data []byte) error {
		if tt != BlocksMsg {
			return fmt.Errorf("check type err: want=%d, got=%d", BlocksMsg, tt)
		}
		var resultJsonObj RemoteBlocks
		err := json.Unmarshal(data, &resultJsonObj)
		if err != nil {
			return err
		}
		if resultJsonObj == nil {
			return fmt.Errorf("data result empty")
		}
		if len(resultJsonObj) != len(wantHashes) {
			return fmt.Errorf("check result lenght err: want:%d, got=%d", len(wantHashes), len(resultJsonObj))
		}
		for i, want := range wantHashes {
			var gotBlock *xfsgo.Block
			err = common.Objcopy(resultJsonObj[i], &gotBlock)
			if err != nil {
				return fmt.Errorf("cover result data err")
			}
			gotHash := gotBlock.HeaderHash()
			if !bytes.Equal(want[:], gotHash[:]) {
				return fmt.Errorf("check result hashes err: index=%d, wantHash: %x, gotHash: %x", i, want, gotHash)
			}
		}
		return nil
	})
	reader := newMsgOnceSendTester(testNodes[0].nodeId, testSendTTL)
	go func() {
		_ = reader.SendObject(GetBlocksMsg, wantHashes)
	}()
	err := mgr.handleMsg(send, reader)
	if err != nil && err != errEOF {
		t.Fatal(err)
	}
}

func TestHandleMsg_handleGotBlocks(t *testing.T) {
	chain := newTestChainMgr(testGenesis, common.Address{})
	maxChain := chain.Copy()
	maxChain.NewEmptyBlocks(10)
	txpool := newTestTxPool(maxChain, maxChain.genesis.Header.GasLimit, testGasPrice)
	mgr := newSyncMgrTester(t, maxChain, txpool)
	reader := newMsgOnceSendTester(testNodes[0].nodeId, testSendTTL)
	go func() {
		req := make(RemoteBlocks, 0)
		lastHeight := maxChain.last.Height()
		for last := maxChain.GetBlockByNumber(lastHeight); last != nil; last = maxChain.GetBlockByHash(last.HashPrevBlock()) {
			remote := coverBlock2RemoteBlock(last)
			req = append(req, remote)
		}
		_ = reader.SendObject(BlocksMsg, req)
	}()
	err := mgr.handleMsg(reader, reader)
	if err != nil && err != errEOF {
		t.Fatal(err)
	}
}

func TestHandleMsg_handleNewBlock(t *testing.T) {
	chain := newTestChainMgr(testGenesis, common.Address{})
	maxChain := chain.Copy()
	maxChain.NewEmptyBlocks(10)
	txpool := newTestTxPool(maxChain, maxChain.genesis.Header.GasLimit, testGasPrice)
	mgr := newSyncMgrTester(t, maxChain, txpool)
	reader := newMsgOnceSendTester(testNodes[0].nodeId, testSendTTL)
	go func() {
		req := coverBlock2RemoteBlock(maxChain.last)
		_ = reader.SendObject(NewBlockMsg, req)
	}()
	err := mgr.handleMsg(reader, reader)
	if err != nil && err != errEOF {
		if err != errUnKnowPeer {
			t.Fatal(err)
		}
	}
}

func TestHandleMsg_handleTransactions(t *testing.T) {
	chain := newTestChainMgr(testGenesis, common.Address{})
	maxChain := chain.Copy()
	maxChain.NewBlock([]*xfsgo.Transaction{
		xfsgo.NewTransactionByStdAndSign(&xfsgo.StdTransaction{
			Version:  0,
			To:       common.Address{},
			GasLimit: maxChain.genesis.Header.GasLimit,
			GasPrice: testGasPrice,
			Value:    new(big.Int).SetInt64(0),
		}, crypto.MustGenPrvKey()),
	}, []*xfsgo.Receipt{
		{
			Version: 0,
			Status:  1,
			TxHash:  common.Hash{},
			GasUsed: new(big.Int).SetInt64(0),
		},
	})
	txpool := newTestTxPool(maxChain, maxChain.genesis.Header.GasLimit, testGasPrice)
	mgr := newSyncMgrTester(t, maxChain, txpool)
	reader := newMsgOnceSendTester(testNodes[0].nodeId, testSendTTL)
	go func() {
		req := coverTxs2RemoteBlockTxs(maxChain.last.Transactions)
		_ = reader.SendObject(TxMsg, req)
	}()
	err := mgr.handleMsg(reader, reader)
	if err != nil && err != errEOF {
		t.Fatal(err)
	}
}

func TestHandleMsg_handleGetReceipts(t *testing.T) {
	chain := newTestChainMgr(testGenesis, common.Address{})
	maxChain := chain.Copy()
	maxChain.NewBlock([]*xfsgo.Transaction{
		xfsgo.NewTransactionByStdAndSign(&xfsgo.StdTransaction{
			Version:  0,
			To:       common.Address{},
			GasLimit: maxChain.genesis.Header.GasLimit,
			GasPrice: testGasPrice,
			Value:    new(big.Int).SetInt64(0),
		}, crypto.MustGenPrvKey()),
		xfsgo.NewTransactionByStdAndSign(&xfsgo.StdTransaction{
			Version:  0,
			To:       common.Address{},
			GasLimit: maxChain.genesis.Header.GasLimit,
			GasPrice: testGasPrice,
			Value:    new(big.Int).SetInt64(0),
		}, crypto.MustGenPrvKey()),
		xfsgo.NewTransactionByStdAndSign(&xfsgo.StdTransaction{
			Version:  0,
			To:       common.Address{},
			GasLimit: maxChain.genesis.Header.GasLimit,
			GasPrice: testGasPrice,
			Value:    new(big.Int).SetInt64(0),
		}, crypto.MustGenPrvKey()),
	}, []*xfsgo.Receipt{
		{
			Version: 0,
			Status:  1,
			TxHash:  common.Bytes2Hash(new(big.Int).SetUint64(0x01).Bytes()),
			GasUsed: new(big.Int).SetInt64(0),
		},
		{
			Version: 0,
			Status:  1,
			TxHash:  common.Bytes2Hash(new(big.Int).SetUint64(0x02).Bytes()),
			GasUsed: new(big.Int).SetInt64(0),
		},
		{
			Version: 0,
			Status:  1,
			TxHash:  common.Bytes2Hash(new(big.Int).SetUint64(0x03).Bytes()),
			GasUsed: new(big.Int).SetInt64(0),
		},
	})
	wantTxHashes := make([]common.Hash, 0)
	for _, re := range maxChain.last.Receipts {
		wantTxHashes = append(wantTxHashes, re.TxHash)
	}
	txpool := newTestTxPool(maxChain, maxChain.genesis.Header.GasLimit, testGasPrice)
	mgr := newSyncMgrTester(t, maxChain, txpool)
	send := newResultCheckSender(func(tt uint8, data []byte) error {
		if tt != ReceiptsData {
			return fmt.Errorf("check type err: want=%d, got=%d", ReceiptsData, tt)
		}
		var args ReceiptsSet
		err := json.Unmarshal(data, &args)
		if err != nil {
			return err
		}
		for i, want := range wantTxHashes {
			gotHash := args[i].TxHash
			if !bytes.Equal(want[:], gotHash[:]) {
				return fmt.Errorf("check result hashes err: index=%d, wantHash: %x, gotHash: %x", i, want, gotHash)
			}
		}
		return nil
	})
	reader := newMsgOnceSendTester(testNodes[0].nodeId, testSendTTL)
	go func() {
		_ = reader.SendObject(GetReceipts, wantTxHashes)
	}()
	err := mgr.handleMsg(send, reader)
	if err != nil && err != errEOF {
		t.Fatal(err)
	}
}
