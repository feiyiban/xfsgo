package dpos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
	"xfsgo"

	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
	"xfsgo/lru"
	"xfsgo/params"

	"github.com/sirupsen/logrus"

	"xfsgo/avlmerkle"
	"xfsgo/common"
	"xfsgo/storage/badger"

	"xfsgo/crypto"
)

const (
	extraVanity        = 32   // Fixed number of extra-data prefix bytes reserved for signer vanity
	extraSeal          = 65   // Fixed number of extra-data suffix bytes reserved for signer seal
	inmemorySignatures = 4096 // Number of recent block signatures to keep in memory

	blockInterval    = int64(10)
	epochInterval    = int64(86400)
	maxValidatorSize = 21
	safeSize         = maxValidatorSize*2/3 + 1
	consensusSize    = maxValidatorSize*2/3 + 1
)

var (
	big0  = big.NewInt(0)
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)

	frontierBlockReward  *big.Int = big.NewInt(5e+18) // Block reward in wei for successfully mining a block
	byzantiumBlockReward *big.Int = big.NewInt(3e+18) // Block reward in wei for successfully mining a block upward from Byzantium

	timeOfFirstBlock = int64(0)

	confirmedBlockHead = []byte("confirmed-block-head")
)

var (
	// errUnknownBlock is returned when the list of signers is requested for a block
	// that is not part of the local blockchain.
	errUnknownBlock = errors.New("unknown block")
	// errMissingVanity is returned if a block's extra-data section is shorter than
	// 32 bytes, which is required to store the signer vanity.
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")
	// errMissingSignature is returned if a block's extra-data section doesn't seem
	// to contain a 65 byte secp256k1 signature.
	errMissingSignature = errors.New("extra-data 65 byte suffix signature missing")
	// errInvalidMixDigest is returned if a block's mix digest is non-zero.
	errInvalidMixDigest = errors.New("non-zero mix digest")
	// errInvalidUncleHash is returned if a block contains an non-empty uncle list.
	errInvalidUncleHash  = errors.New("non empty uncle hash")
	errInvalidDifficulty = errors.New("invalid difficulty")

	// ErrInvalidTimestamp is returned if the timestamp of a block is lower than
	// the previous block's timestamp + the minimum block period.
	ErrInvalidTimestamp           = errors.New("invalid timestamp")
	ErrWaitForPrevBlock           = errors.New("wait for last block arrived")
	ErrMintFutureBlock            = errors.New("mint the future block")
	ErrMismatchSignerAndValidator = errors.New("mismatch block signer and validator")
	ErrInvalidBlockValidator      = errors.New("invalid block validator")
	ErrInvalidMintBlockTime       = errors.New("invalid time to mint the block")
	ErrNilBlockHeader             = errors.New("nil block header returned")
)

type Dpos struct {
	config *params.DposConfig // Consensus engine configuration parameters
	db     *badger.Storage    // Database to store and retrieve snapshot checkpoints

	signer               common.Address
	signFn               SignerFn
	signatures           *lru.Cache // Signatures of recent blocks to speed up mining
	confirmedBlockHeader *xfsgo.BlockHeader

	mu   sync.RWMutex
	stop chan bool
}

type SignerFn func(*xfsgo.StateObj, []byte) ([]byte, error)

//注：sigHash为集团复制品
//sigHash返回哈希，该哈希用作权限证明的输入
//签字。它是除65字节签名之外的整个头的散列
//包含在额外数据的末尾。
//
//注意，该方法要求额外数据至少为65字节，否则
//恐慌。这样做是为了避免意外地使用这两种表格（签名）
//（或否），可能会被滥用为同一标头生成不同的哈希。
func sigHash(header *xfsgo.BlockHeader) common.Hash {
	// hasher := sha3.NewKeccak256()
	bs, _ := rawencode.Encode(header)
	// hasher.Write(bs)
	info := ahash.SHA256(bs)
	return common.Bytes2Hash(info)
}

func New(config *params.DposConfig, db *badger.Storage) *Dpos {
	signatures := lru.NewCache(inmemorySignatures)

	return &Dpos{
		config:     config,
		db:         db,
		signatures: signatures,
	}
}

func (d *Dpos) Author(header *xfsgo.BlockHeader) (common.Address, error) {
	return header.Validator, nil
}

func (d *Dpos) Coinbase(header *xfsgo.BlockHeader) (common.Address, error) {
	return header.Coinbase, nil
}

func (d *Dpos) VerifyHeader(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader, seal bool) error {
	return d.verifyHeader(chain, header, nil)
}

func (d *Dpos) verifyHeader(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader, parents []*xfsgo.BlockHeader) error {
	if header.Number() == nil {
		return errors.New("unknown block")
	}
	number := header.Number().Uint64()
	// Unnecssary to verify the block from feature
	if header.Time().Cmp(big.NewInt(time.Now().Unix())) > 0 {
		return errors.New("block in the future")
	}
	// Check that the extra-data contains both the vanity and signature
	if len(header.Extra) < extraVanity {
		return errors.New("extra-data 32 byte vanity prefix missing")
	}
	if len(header.Extra) < extraVanity+extraSeal {
		return errors.New("extra-data 65 byte suffix signature missing")
	}
	// Ensure that the mix digest is zero as we don't have fork protection currently
	if header.MixDigest != (common.Hash{}) {
		return errors.New("non-zero mix digest")
	}
	// Difficulty always 1
	if header.Difficulty.Uint64() != 1 {
		errors.New("invalid difficulty")
	}
	// Ensure that the block doesn't contain any uncles which are meaningless in DPoS
	// if header.UncleHash != uncleHash {
	// 	return errInvalidUncleHash
	// }
	// If all checks passed, validate any special fields for hard forks
	// if err := misc.VerifyForkHashes(chain.Config(), header, false); err != nil {
	// 	return err
	// }

	var parent *xfsgo.BlockHeader
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetBlockByHash(header.HeaderHash()).Header
		// parent = chain.GetHeader(header.ParentHash, number-1)
	}
	if parent == nil || parent.Number().Uint64() != number-1 || parent.HashHex() != header.HashHex() {
		return errors.New("unknown ancestor")
	}
	if parent.Time().Uint64()+uint64(blockInterval) > header.Time().Uint64() {
		return ErrInvalidTimestamp
	}
	return nil
}

func (d *Dpos) VerifyHeaders(chain xfsgo.IBlockChain, headers []*xfsgo.BlockHeader, seals []bool) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := d.verifyHeader(chain, header, headers[:i])
			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// VerifyUncles implements consensus.Engine, always returning an error for any
// uncles as this consensus mechanism doesn't permit uncles.
// func (d *Dpos) VerifyUncles(chain xfsgo.IBlockChain, block *xfsgo.Block) error {
// 	if len(block.Uncles()) > 0 {
// 		return errors.New("uncles not allowed")
// 	}
// 	return nil
// }

// VerifySeal implements consensus.Engine, checking whether the signature contained
// in the header satisfies the consensus protocol requirements.
func (d *Dpos) VerifySeal(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader) error {
	return d.verifySeal(chain, header, nil)
}

func (d *Dpos) verifySeal(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader, parents []*xfsgo.BlockHeader) error {
	// Verifying the genesis block is not supported
	number := header.Number().Uint64()
	if number == 0 {
		return errUnknownBlock
	}
	var parent *xfsgo.BlockHeader
	if len(parents) > 0 {
		parent = parents[len(parents)-1]
	} else {
		parent = chain.GetBlockByHash(header.HeaderHash()).Header
	}
	dposContext, err := avlmerkle.NewDposContextFromProto(d.db, parent.DposContext)
	if err != nil {
		return err
	}
	epochContext := &EpochContext{DposContext: dposContext}
	validator, err := epochContext.lookupValidator(header.Time().Int64())
	if err != nil {
		return err
	}
	if err := d.verifyBlockSigner(validator, header); err != nil {
		return err
	}
	return d.updateConfirmedBlockHeader(chain)
}

func (d *Dpos) verifyBlockSigner(validator common.Address, header *xfsgo.BlockHeader) error {
	signer, err := ecrecover(header, d.signatures)
	if err != nil {
		return err
	}
	if bytes.Compare(signer.Bytes(), validator.Bytes()) != int(0) {
		return ErrInvalidBlockValidator
	}
	if bytes.Compare(signer.Bytes(), header.Validator.Bytes()) != int(0) {
		return ErrMismatchSignerAndValidator
	}
	return nil
}

func (d *Dpos) updateConfirmedBlockHeader(chain xfsgo.IBlockChain) error {
	if d.confirmedBlockHeader == nil {
		header, err := d.loadConfirmedBlockHeader(chain)
		if err != nil {
			header = chain.GetBlockByNumber(uint64(0)).Header
			if header == nil {
				return err
			}
		}
		d.confirmedBlockHeader = header
	}

	curHeader := chain.CurrentBHeader()
	epoch := int64(-1)
	validatorMap := make(map[common.Address]bool)
	for d.confirmedBlockHeader.HashHex() != curHeader.HashHex() &&
		d.confirmedBlockHeader.Number().Uint64() < curHeader.Number().Uint64() {
		curEpoch := curHeader.Time().Int64() / epochInterval
		if curEpoch != epoch {
			epoch = curEpoch
			validatorMap = make(map[common.Address]bool)
		}
		// fast return
		// if block number difference less consensusSize-witnessNum
		// there is no need to check block is confirmed
		if curHeader.Number().Int64()-d.confirmedBlockHeader.Number().Int64() < int64(consensusSize-len(validatorMap)) {
			logrus.Debug("Dpos fast return", "current", curHeader.Number().String(), "confirmed", d.confirmedBlockHeader.Number().String(), "witnessCount", len(validatorMap))
			return nil
		}

		validatorMap[curHeader.Validator] = true
		if len(validatorMap) >= consensusSize {
			d.confirmedBlockHeader = curHeader
			if err := d.storeConfirmedBlockHeader(d.db); err != nil {
				return err
			}
			logrus.Debug("dpos set confirmed block header success", "currentHeader", curHeader.Number().String())
			return nil
		}
		curHeader = chain.GetBlockByHash(curHeader.HeaderHash()).Header
		if curHeader == nil {
			return ErrNilBlockHeader
		}
	}
	return nil
}

func (s *Dpos) loadConfirmedBlockHeader(chain xfsgo.IBlockChain) (*xfsgo.BlockHeader, error) {
	key, err := s.db.GetData(confirmedBlockHead)
	if err != nil {
		return nil, err
	}
	header := chain.GetBlockByHash(common.Bytes2Hash(key)).Header
	if header == nil {
		return nil, ErrNilBlockHeader
	}
	return header, nil
}

// store inserts the snapshot into the database.
func (s *Dpos) storeConfirmedBlockHeader(db badger.IStorage) error {
	scHash := s.confirmedBlockHeader.HeaderHash()
	return db.SetData(confirmedBlockHead, scHash.Bytes())
}

func (d *Dpos) Prepare(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader) error {
	header.Nonce = 0
	// number := header.Number().Uint64()
	if len(header.Extra) < extraVanity {
		header.Extra = append(header.Extra, bytes.Repeat([]byte{0x00}, extraVanity-len(header.Extra))...)
	}
	header.Extra = header.Extra[:extraVanity]
	header.Extra = append(header.Extra, make([]byte, extraSeal)...)
	parent := chain.GetBlockByNumber(header.Height)

	if parent == nil {
		return errors.New("unknown ancestor")
	}
	headerParent := parent.Header

	header.Difficulty = d.CalcDifficulty(chain, header.Time().Uint64(), headerParent)
	header.Validator = d.signer
	return nil
}

func AccumulateRewards(config *params.ChainConfig, state *xfsgo.StateTree, header *xfsgo.BlockHeader) {
	// Select the correct block reward based on chain progression
	blockReward := frontierBlockReward
	if config.IsByzantium(header.Number()) {
		blockReward = byzantiumBlockReward
	}
	// Accumulate the rewards for the miner and any included uncles
	reward := new(big.Int).Set(blockReward)
	state.AddBalance(header.Coinbase, reward)
}

func (d *Dpos) Finalize(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader, state *xfsgo.StateTree, txs []*xfsgo.Transaction, receipts []*xfsgo.Receipt, dposContext *avlmerkle.DposContext) (*xfsgo.Block, error) {
	// Accumulate block rewards and commit the final state root
	AccumulateRewards(chain.Config(), state, header)
	// header.StateRoot = state.IntermediateRoot(chain.Config().IsEIP158(header.Number()))
	// header.StateRoot = statfunc Example() {
	//    header.
	//Output:
	// header header.Root()

	// }
	parent := chain.GetBlockByHash(header.HeaderHash()).Header
	epochContext := &EpochContext{
		statedb:     state,
		DposContext: dposContext,
		TimeStamp:   header.Time().Int64(),
	}
	if timeOfFirstBlock == 0 {
		if firstBlockHeader := chain.GetBlockByNumber(1).Header; firstBlockHeader != nil {
			timeOfFirstBlock = firstBlockHeader.Time().Int64()
		}
	}
	genesis := chain.GetBlockByNumber(uint64(0)).Header
	err := epochContext.tryElect(genesis, parent)
	if err != nil {
		return nil, fmt.Errorf("got error when elect next epoch, err: %s", err)
	}

	//update mint count trie
	updateMintCnt(parent.Time().Int64(), header.Time().Int64(), header.Validator, dposContext)
	header.DposContext = dposContext.ToProto()
	return xfsgo.NewBlock(header, txs, receipts), nil
}

func (d *Dpos) checkDeadline(lastBlock *xfsgo.Block, now int64) error {
	prevSlot := PrevSlot(now)
	nextSlot := NextSlot(now)
	if lastBlock.GetHeader().Time().Int64() >= nextSlot {
		return ErrMintFutureBlock
	}
	// last block was arrived, or time's up
	if lastBlock.GetHeader().Time().Int64() == prevSlot || nextSlot-now <= 1 {
		return nil
	}
	return ErrWaitForPrevBlock
}

func (d *Dpos) CheckValidator(lastBlock *xfsgo.Block, now int64) error {
	if err := d.checkDeadline(lastBlock, now); err != nil {
		return err
	}
	dposContext, err := avlmerkle.NewDposContextFromProto(d.db, lastBlock.Header.DposContext)
	if err != nil {
		return err
	}
	epochContext := &EpochContext{DposContext: dposContext}
	validator, err := epochContext.lookupValidator(now)
	if err != nil {
		return err
	}
	if (validator == common.Address{}) || bytes.Compare(validator.Bytes(), d.signer.Bytes()) != 0 {
		return ErrInvalidBlockValidator
	}
	return nil
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (d *Dpos) Seal(chain xfsgo.IBlockChain, block *xfsgo.Block, stop <-chan struct{}) (*xfsgo.Block, error) {
	header := block.GetHeader()
	number := header.Number().Uint64()
	// Sealing the genesis block is not supported
	if number == 0 {
		return nil, errUnknownBlock
	}
	now := time.Now().Unix()
	delay := NextSlot(now) - now
	if delay > 0 {
		select {
		case <-stop:
			return nil, nil
		case <-time.After(time.Duration(delay) * time.Second):
		}
	}
	block.GetHeader().Time().SetInt64(time.Now().Unix())

	// time's up, sign the block
	headsig := sigHash(header)
	tree := chain.CurrentStateTree()
	signAccount := xfsgo.NewStateObjN(d.signer, tree)
	sighash, err := d.signFn(signAccount, headsig.Bytes())
	if err != nil {
		return nil, err
	}
	copy(header.Extra[len(header.Extra)-extraSeal:], sighash)
	return block.WithSeal(header), nil
}

func (d *Dpos) CalcDifficulty(chain xfsgo.IBlockChain, time uint64, parent *xfsgo.BlockHeader) *big.Int {
	return big.NewInt(1)
}

func (d *Dpos) APIs(chain xfsgo.IBlockChain) error {
	// return []rpc.API{{
	// 	Namespace: "dpos",
	// 	Version:   "1.0",
	// 	Service:   &API{chain: chain, dpos: d},
	// 	Public:    true,
	// }}
	return nil
}

func (d *Dpos) Authorize(signer common.Address, signFn SignerFn) {
	d.mu.Lock()
	d.signer = signer
	d.signFn = signFn
	d.mu.Unlock()
}

// ecrecover extracts the Ethereum account address from a signed header.
func ecrecover(header *xfsgo.BlockHeader, sigcache *lru.Cache) (common.Address, error) {
	// If the signature's already cached, return that
	hash := header.HeaderHash()
	if address, known := sigcache.Get(hash); known {
		// return address.(common.Address), nil
		return common.Bytes2Address(address), nil
	}
	// Retrieve the signature from the header extra-data
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]
	// Recover the public key and the Ethereum address
	sigHash := sigHash(header)
	pubkey, err := crypto.Ecrecover(sigHash.Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address

	copy(signer[:], ahash.SHA256(pubkey[1:])[12:])
	// sigcache.Add(hash, signer)
	sigcache.Put(hash, signer.Bytes())
	return signer, nil
}

func PrevSlot(now int64) int64 {
	return int64((now-1)/blockInterval) * blockInterval
}

func NextSlot(now int64) int64 {
	return int64((now+blockInterval-1)/blockInterval) * blockInterval
}

// //更新新锁矿工的MintCntTrie中的计数
func updateMintCnt(parentBlockTime, currentBlockTime int64, validator common.Address, dposContext *avlmerkle.DposContext) {
	currentMintCntTrie := dposContext.MintCntTrie()
	currentEpoch := parentBlockTime / epochInterval
	currentEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(currentEpochBytes, uint64(currentEpoch))

	cnt := int64(1)
	newEpoch := currentBlockTime / epochInterval
	// still during the currentEpochID
	if currentEpoch == newEpoch {

		// currentMintCntTrie
		currentMintCntTrie.Foreach(func(k, v []byte) {
			// key := append(k, d.delegateTrie.prefix...)
			cntBytes, ok := currentMintCntTrie.Get(append(currentEpochBytes, validator.Bytes()...))

			// not the first time to mint
			if cntBytes != nil && ok {
				cnt = int64(binary.BigEndian.Uint64(cntBytes)) + 1
			}
		})
		// iter := trie.NewIterator(currentMintCntTrie.NodeIterator(currentEpochBytes))

		// // 当当前不是genesis时，从MintCntTrie读取上次计数
		// if iter.Next() {
		// 	cntBytes := currentMintCntTrie.Get(append(currentEpochBytes, validator.Bytes()...))

		// 	// not the first time to mint
		// 	if cntBytes != nil {
		// 		cnt = int64(binary.BigEndian.Uint64(cntBytes)) + 1
		// 	}
		// }
	}

	newCntBytes := make([]byte, 8)
	newEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(newEpochBytes, uint64(newEpoch))
	binary.BigEndian.PutUint64(newCntBytes, uint64(cnt))
	dposContext.MintCntTrie().Update(append(newEpochBytes, validator.Bytes()...), newCntBytes)
}
