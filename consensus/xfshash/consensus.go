package xfshash

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"
	"xfsgo"
	"xfsgo/avlmerkle"
	"xfsgo/common"
	"xfsgo/params"
)

// Ethash proof-of-work protocol constants.
var (
	frontierBlockReward  *big.Int = big.NewInt(5e+18) //魏成功挖掘区块的区块奖励
	byzantiumBlockReward *big.Int = big.NewInt(3e+18) //魏成功从拜占庭向上开采区块的区块奖励
	// maxUncles                     = 2                 // Maximum number of uncles allowed in a single block
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
// var (
// 	errLargeBlockTime    = errors.New("timestamp too big")
// 	errZeroBlockTime     = errors.New("timestamp equals parent's")
// 	errTooManyUncles     = errors.New("too many uncles")
// 	errDuplicateUncle    = errors.New("duplicate uncle")
// 	errUncleIsAncestor   = errors.New("uncle is ancestor")
// 	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
// 	errNonceOutOfRange   = errors.New("nonce out of range")
// 	errInvalidDifficulty = errors.New("non-positive difficulty")
// 	errInvalidMixDigest  = errors.New("invalid mix digest")
// 	errInvalidPoW        = errors.New("invalid proof-of-work")
// )

// Author implements consensus.Engine, returning the header's coinbase as the
// proof-of-work verified author of the block.
func (ethash *Ethash) Author(header *xfsgo.BlockHeader) (common.Address, error) {
	return header.Coinbase, nil
}

func (ethash *Ethash) Coinbase(header *xfsgo.BlockHeader) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// stock Ethereum ethash engine.
func (ethash *Ethash) VerifyHeader(chain *xfsgo.BlockChain, header *xfsgo.BlockHeader, seal bool) error {
	// If we're running a full engine faking, accept any input as valid
	if ethash.fakeFull {
		return nil
	}
	// Short circuit if the header is known, or it's parent not
	number := header.Number().Uint64()
	if chain.GetBlockByNumber(number) != nil {
		return nil
	}
	parent := chain.GetBlockByNumber(number)
	if parent == nil {
		return errors.New("unknown ancestor")
	}
	// Sanity checks passed, do a proper verification
	return ethash.verifyHeader(chain, header, parent.Header, false, seal)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (ethash *Ethash) VerifyHeaders(chain *xfsgo.BlockChain, headers []*xfsgo.BlockHeader, seals []bool) (chan<- struct{}, <-chan error) {
	// If we're running a full engine faking, accept any input as valid
	if ethash.fakeFull || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}

	// Spawn as many workers as allowed threads
	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}

	// Create a task channel and spawn the verifiers
	var (
		inputs = make(chan int)
		done   = make(chan int, workers)
		errors = make([]error, len(headers))
		abort  = make(chan struct{})
	)
	for i := 0; i < workers; i++ {
		go func() {
			for index := range inputs {
				errors[index] = ethash.verifyHeaderWorker(chain, headers, seals, index)
				done <- index
			}
		}()
	}

	errorsOut := make(chan error, len(headers))
	go func() {
		defer close(inputs)
		var (
			in, out = 0, 0
			checked = make([]bool, len(headers))
			inputs  = inputs
		)
		for {
			select {
			case inputs <- in:
				if in++; in == len(headers) {
					// Reached end of headers. Stop sending to workers.
					inputs = nil
				}
			case index := <-done:
				for checked[index] = true; checked[out]; out++ {
					errorsOut <- errors[out]
					if out == len(headers)-1 {
						return
					}
				}
			case <-abort:
				return
			}
		}
	}()
	return abort, errorsOut
}

func (ethash *Ethash) verifyHeaderWorker(chain *xfsgo.BlockChain, headers []*xfsgo.BlockHeader, seals []bool, index int) error {
	var parent *xfsgo.BlockHeader
	if index == 0 {
		parent = chain.GetBlockByHash(headers[0].HeaderHash()).Header
	} else if headers[index-1].HashHex() == headers[index].HashHex() {
		parent = headers[index-1]
	}
	if parent == nil {
		return errors.New("unknown ancestor")
	}
	if chain.GetBlockByHash(headers[index].HeaderHash()) != nil {
		return nil // known block
	}
	return ethash.verifyHeader(chain, headers[index], parent, false, seals[index])
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of the stock Ethereum ethash engine.
// func (ethash *Ethash) VerifyUncles(chain *xfsgo.BlockChain, block *types.Block) error {
// 	// If we're running a full engine faking, accept any input as valid
// 	if ethash.fakeFull {
// 		return nil
// 	}
// 	// Verify that there are at most 2 uncles included in this block
// 	// if len(block.Uncles()) > maxUncles {
// 	// 	return errTooManyUncles
// 	// }
// 	// Gather the set of past uncles and ancestors
// 	uncles, ancestors := set.New(), make(map[common.Hash]*xfsgo.BlockHeader)

// 	number := block.NumberU64() - 1
// 	for i := 0; i < 7; i++ {
// 		ancestor := chain.GetBlockByHash(number)
// 		if ancestor == nil {
// 			break
// 		}
// 		ancestors[ancestor.HeaderHash()] = ancestor.Header
// 		// for _, uncle := range ancestor.Uncles() {
// 		// 	uncles.Add(uncle.Hash())
// 		// }
// 		number = number - 1
// 	}
// 	ancestors[block.Hash()] = block.Header()
// 	uncles.Add(block.Hash())

// 	// Verify each of the uncles that it's recent, but not an ancestor
// 	// for _, uncle := range block.Uncles() {
// 	// 	// Make sure every uncle is rewarded only once
// 	// 	hash := uncle.Hash()
// 	// 	if uncles.Has(hash) {
// 	// 		return errDuplicateUncle
// 	// 	}
// 	// 	uncles.Add(hash)

// 	// 	// Make sure the uncle has a valid ancestry
// 	// 	if ancestors[hash] != nil {
// 	// 		return errUncleIsAncestor
// 	// 	}
// 	// 	if ancestors[uncle.ParentHash] == nil || uncle.ParentHash == block.ParentHash() {
// 	// 		return errDanglingUncle
// 	// 	}
// 	// 	if err := ethash.verifyHeader(chain, uncle, ancestors[uncle.ParentHash], true, true); err != nil {
// 	// 		return err
// 	// 	}
// 	// }
// 	return nil
// }

// verifyHeader checks whether a header conforms to the consensus rules of the
// stock Ethereum ethash engine.
// See YP section 4.3.4. "Block Header Validity"
func (ethash *Ethash) verifyHeader(chain *xfsgo.BlockChain, header, parent *xfsgo.BlockHeader, uncle bool, seal bool) error {
	// Ensure that the header's extra-data section is of a reasonable size
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
	// Verify the header's timestamp
	if uncle {
		if header.Time().Cmp(common.BigMaxUint256) > 0 {
			return errors.New("timestamp too big")
		}
	} else {
		if header.Time().Cmp(big.NewInt(time.Now().Unix())) > 0 {
			return errors.New("block in the future")
		}
	}
	if header.Time().Cmp(parent.Time()) <= 0 {
		return errors.New("timestamp equals parent's")
	}
	// Verify the block's difficulty based in it's timestamp and parent's difficulty
	expected := CalcDifficulty(chain.Config(), header.Time().Uint64(), parent)
	if expected.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
	}
	// Verify that the gas limit is <= 2^63-1
	if header.GasLimit.Cmp(common.BigMaxUint64) > 0 {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, common.BigMaxUint64)
	}
	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed.Cmp(header.GasLimit) > 0 {
		return fmt.Errorf("invalid gasUsed: have %v, gasLimit %v", header.GasUsed, header.GasLimit)
	}

	// Verify that the gas limit remains within allowed bounds
	diff := new(big.Int).Set(parent.GasLimit)
	diff = diff.Sub(diff, header.GasLimit)
	diff.Abs(diff)

	limit := new(big.Int).Set(parent.GasLimit)
	// limit = limit.Div(limit, common.GasLimitBoundDivisor)

	if diff.Cmp(limit) >= 0 || header.GasLimit.Cmp(params.MinGasLimit) < 0 {
		return fmt.Errorf("invalid gas limit: have %v, want %v += %v", header.GasLimit, parent.GasLimit, limit)
	}
	// Verify that the block number is parent's +1
	if diff := new(big.Int).Sub(header.Number(), parent.Number()); diff.Cmp(big.NewInt(1)) != 0 {
		return errors.New("invalid block number")
	}
	// Verify the engine specific seal securing the block
	if seal {
		if err := ethash.VerifySeal(chain, header); err != nil {
			return err
		}
	}
	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
// TODO (karalabe): Move the chain maker into this package and make this private!
func CalcDifficulty(config *params.ChainConfig, time uint64, parent *xfsgo.BlockHeader) *big.Int {
	next := new(big.Int).Add(parent.Number(), big1)
	switch {
	case config.IsByzantium(next):
		return calcDifficultyByzantium(time, parent)
	case config.IsHomestead(next):
		return calcDifficultyHomestead(time, parent)
	default:
		return calcDifficultyFrontier(time, parent)
	}
}

// Some weird constants to avoid constant memory allocs for them.
var (
	expDiffPeriod = big.NewInt(100000)
	big1          = big.NewInt(1)
	big2          = big.NewInt(2)
	big9          = big.NewInt(9)
	big10         = big.NewInt(10)
	bigMinus99    = big.NewInt(-99)
	big2999999    = big.NewInt(2999999)
)

// calcDifficultyByzantium is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time given the
// parent block's time and difficulty. The calculation uses the Byzantium rules.
func calcDifficultyByzantium(time uint64, parent *xfsgo.BlockHeader) *big.Int {
	// https://github.com/ethereum/EIPs/issues/100.
	// algorithm:
	// diff = (parent_diff +
	//         (parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
	//        ) + 2^(periodCount - 2)

	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).SetUint64(parent.Timestamp)

	// holds intermediate values to make the algo easier to read & audit
	x := new(big.Int)
	y := new(big.Int)

	// (2 if len(parent_uncles) else 1) - (block_timestamp - parent_timestamp) // 9
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big9)
	// if parent.UncleHash == types.EmptyUncleHash {
	// x.Sub(big1, x)
	// } else {
	x.Sub(big2, x)
	// }
	// max((2 if len(parent_uncles) else 1) - (block_timestamp - parent_timestamp) // 9, -99)
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	// (parent_diff + parent_diff // 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)
	// minimum difficulty can ever be (before exponential factor)
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
	// calculate a fake block numer for the ice-age delay:
	//   https://github.com/ethereum/EIPs/pull/669
	//   fake_block_number = min(0, block.number - 3_000_000
	fakeBlockNumber := new(big.Int)
	if parent.Number().Cmp(big2999999) >= 0 {
		fakeBlockNumber = fakeBlockNumber.Sub(parent.Number(), big2999999) // Note, parent is 1 less than the actual block number
	}
	// for the exponential factor
	periodCount := fakeBlockNumber
	periodCount.Div(periodCount, expDiffPeriod)

	// the exponential factor, commonly referred to as "the bomb"
	// diff = diff + 2^(periodCount - 2)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}

// calcDifficultyHomestead is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time given the
// parent block's time and difficulty. The calculation uses the Homestead rules.
func calcDifficultyHomestead(time uint64, parent *xfsgo.BlockHeader) *big.Int {
	// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-2.mediawiki
	// algorithm:
	// diff = (parent_diff +
	//         (parent_diff / 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	//        ) + 2^(periodCount - 2)

	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time())

	// holds intermediate values to make the algo easier to read & audit
	x := new(big.Int)
	y := new(big.Int)

	// 1 - (block_timestamp - parent_timestamp) // 10
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big10)
	x.Sub(big1, x)

	// max(1 - (block_timestamp - parent_timestamp) // 10, -99)
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	// (parent_diff + parent_diff // 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)

	// minimum difficulty can ever be (before exponential factor)
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
	// for the exponential factor
	periodCount := new(big.Int).Add(parent.Number(), big1)
	periodCount.Div(periodCount, expDiffPeriod)

	// the exponential factor, commonly referred to as "the bomb"
	// diff = diff + 2^(periodCount - 2)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}

//calcDifficultyFrontier是难度调整算法。它返回
//在给定父对象的时间创建新块时应具有的难度
//区块的时间和困难。计算使用边界规则。
func calcDifficultyFrontier(time uint64, parent *xfsgo.BlockHeader) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, params.DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.Set(parent.Time())

	if bigTime.Sub(bigTime, bigParentTime).Cmp(params.DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	if diff.Cmp(params.MinimumDifficulty) < 0 {
		diff.Set(params.MinimumDifficulty)
	}

	periodCount := new(big.Int).Add(parent.Number(), big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		// diff = diff + 2^(periodCount - 2)
		expDiff := periodCount.Sub(periodCount, big2)
		expDiff.Exp(big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = common.BigMax(diff, params.MinimumDifficulty)
	}
	return diff
}

// VerifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (ethash *Ethash) VerifySeal(chain *xfsgo.BlockChain, header *xfsgo.BlockHeader) error {
	// If we're running a fake PoW, accept any seal as valid
	if ethash.fakeMode {
		time.Sleep(ethash.fakeDelay)
		if ethash.fakeFail == header.Number().Uint64() {
			return errors.New("invalid proof-of-work")
		}
		return nil
	}
	// If we're running a shared PoW, delegate verification to it
	if ethash.shared != nil {
		return ethash.shared.VerifySeal(chain, header)
	}
	// Sanity check that the block number is below the lookup table size (60M blocks)
	number := header.Number().Uint64()
	if number/epochLength >= uint64(len(cacheSizes)) {
		// Go < 1.7 cannot calculate new cache/dataset sizes (no fast prime check)
		return errors.New("nonce out of range")
	}
	// Ensure that we have a valid difficulty for the block
	if header.Difficulty.Sign() <= 0 {
		return errors.New("non-positive difficulty")
	}
	// Recompute the digest and PoW value and verify against the header
	cache := ethash.cache(number)

	size := datasetSize(number)
	if ethash.tester {
		size = 32 * 1024
	}
	HashNoNonce := header.HashNoNonce()
	digest, result := hashimotoLight(size, cache, HashNoNonce.Bytes(), header.HeaderNonce().Uint64())
	if !bytes.Equal(header.MixDigest[:], digest) {
		return errors.New("invalid mix digest")
	}
	target := new(big.Int).Div(maxUint256, header.Difficulty)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return errors.New("invalid proof-of-work")
	}
	return nil
}

// Prepare implements consensus.Engine, initializing the difficulty field of a
// header to conform to the ethash protocol. The changes are done inline.
func (ethash *Ethash) Prepare(chain *xfsgo.BlockChain, header *xfsgo.BlockHeader) error {
	// parent := chain.GetHeader(header.ParentHash, header.Number().Uint64()-1)
	parent := chain.GetBlockByNumber(header.Number().Uint64() - 1)
	if parent == nil {
		return errors.New("unknown ancestor")
	}
	header.Difficulty = CalcDifficulty(chain.Config(), header.Time().Uint64(), parent.Header)

	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (ethash *Ethash) Finalize(chain *xfsgo.BlockChain, header *xfsgo.BlockHeader, state *xfsgo.StateTree, txs []*xfsgo.Transaction, receipts []*xfsgo.Receipt, ctx *avlmerkle.DposContext) (*xfsgo.Block, error) {
	// Accumulate any block and uncle rewards and commit the final state root
	AccumulateRewards(chain.Config(), state, header)
	// header. = state.IntermediateRoot(chain.Config().IsEIP158(header.Number()))

	// Header seems complete, assemble into a block and return
	return xfsgo.NewBlock(header, txs, receipts), nil
}

// Some weird constants to avoid constant memory allocs for them.
var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)

//累计向指定区块的coinbase授予采矿权
//奖励。总奖励包括静态块奖励和
//包括叔叔。每个叔叔区块的硬币库也会得到奖励。
//TODO（卡拉拉贝）：将链制造商移动到此包中，并将其私有化！
func AccumulateRewards(config *params.ChainConfig, state *xfsgo.StateTree, header *xfsgo.BlockHeader) {
	// Select the correct block reward based on chain progression
	blockReward := frontierBlockReward
	if config.IsByzantium(header.Number()) {
		blockReward = byzantiumBlockReward
	}
	// Accumulate the rewards for the miner and any included uncles
	reward := new(big.Int).Set(blockReward)
	/*
		r := new(big.Int)
		for _, uncle := range uncles {
			r.Add(uncle.Number, big8)
			r.Sub(r, header.Number)
			r.Mul(r, blockReward)
			r.Div(r, big8)
			state.AddBalance(uncle.Coinbase, r)

			r.Div(blockReward, big32)
			reward.Add(reward, r)
		}
	*/
	state.AddBalance(header.Coinbase, reward)
}
