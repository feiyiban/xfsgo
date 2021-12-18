package dpos

import (
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"
	"xfsgo"

	"xfsgo/assert"
	"xfsgo/avlmerkle"
	"xfsgo/common"
	"xfsgo/test"
)

func TestEpochContextCountVotes(t *testing.T) {
	voteMap := map[common.Address][]common.Address{
		common.Hex2Address("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e"): {
			common.Hex2Address("0xb040353ec0f2c113d5639444f7253681aecda1f8"),
		},
		common.Hex2Address("0xa60a3886b552ff9992cfcd208ec1152079e046c2"): {
			common.Hex2Address("0x14432e15f21237013017fa6ee90fc99433dec82c"),
			common.Hex2Address("0x9f30d0e5c9c88cade54cd1adecf6bc2c7e0e5af6"),
		},
		common.Hex2Address("0x4e080e49f62694554871e669aeb4ebe17c4a9670"): {
			common.Hex2Address("0xd83b44a3719720ec54cdb9f54c0202de68f1ebcb"),
			common.Hex2Address("0x56cc452e450551b7b9cffe25084a069e8c1e9441"),
			common.Hex2Address("0xbcfcb3fa8250be4f2bf2b1e70e1da500c668377b"),
		},
		common.Hex2Address("0x9d9667c71bb09d6ca7c3ed12bfe5e7be24e2ffe1"): {},
	}
	balance := int64(5)
	db := test.NewMemStorage()

	stateDB := xfsgo.NewStateTree(db, nil)
	dposContext, err := avlmerkle.NewDposContext(db)
	assert.Nil(t, err)

	epochContext := &EpochContext{
		DposContext: dposContext,
		statedb:     stateDB,
	}
	_, err = epochContext.countVotes()
	assert.Nil(t, err)

	for candidate, electors := range voteMap {
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
		for _, elector := range electors {
			stateDB.SetBalance(elector, big.NewInt(balance))
			assert.Nil(t, dposContext.Delegate(elector, candidate))
		}
	}
	result, err := epochContext.countVotes()
	assert.Nil(t, err)
	assert.Equal(t, len(voteMap), len(result))
	for candidate, electors := range voteMap {
		voteCount, ok := result[candidate]
		assert.True(t, ok)
		assert.Equal(t, balance*int64(len(electors)), voteCount.Int64())
	}
}

func TestLookupValidator(t *testing.T) {
	db := test.NewMemStorage()
	dposCtx, _ := avlmerkle.NewDposContext(db)
	mockEpochContext := &EpochContext{
		DposContext: dposCtx,
	}
	validators := []common.Address{
		common.Hex2Address("addr1"),
		common.Hex2Address("addr2"),
		common.Hex2Address("addr3"),
	}
	mockEpochContext.DposContext.SetValidators(validators)
	for i, expected := range validators {
		got, _ := mockEpochContext.lookupValidator(int64(i) * blockInterval)
		if got != expected {
			t.Errorf("Failed to test lookup validator, %s was expected but got %s", expected.B58String(), got.B58String())
		}
	}
	_, err := mockEpochContext.lookupValidator(blockInterval - 1)
	if err != ErrInvalidMintBlockTime {
		t.Errorf("Failed to test lookup validator. err '%v' was expected but got '%v'", ErrInvalidMintBlockTime, err)
	}
}

func TestEpochContextKickoutValidator(t *testing.T) {
	db := test.NewMemStorage()

	stateDB := xfsgo.NewStateTree(db, nil)
	dposContext, err := avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext := &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	atLeastMintCnt := epochInterval / blockInterval / maxValidatorSize / 2
	testEpoch := int64(1)

	// no validator can be kickout, because all validators mint enough block at least
	validators := []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.Hex2Address("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt)
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, dposContext.BecomeCandidate(common.Hex2Address("addr")))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap := getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize+1, len(candidateMap))

	// atLeast a safeSize count candidate will reserve
	dposContext, err = avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.Hex2Address("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-int64(i)-1)
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, safeSize, len(candidateMap))
	for i := maxValidatorSize - 1; i >= safeSize; i-- {
		// assert.False(t, candidateMap[common.Hex2Address("addr"+strconv.Itoa(i))])
	}

	// all validator will be kickout, because all validators didn't mint enough block at least
	dposContext, err = avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.Hex2Address("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
	}
	for i := maxValidatorSize; i < maxValidatorSize*2; i++ {
		candidate := common.Hex2Address("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize, len(candidateMap))

	// only one validator mint count is not enough
	dposContext, err = avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.Hex2Address("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		if i == 0 {
			setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
		} else {
			setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt)
		}
	}
	assert.Nil(t, dposContext.BecomeCandidate(common.Hex2Address("addr")))
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize, len(candidateMap))
	// assert.False(t, candidateMap[common.Hex2Address("addr"+strconv.Itoa(0))])

	// epochTime is not complete, all validators mint enough block at least
	dposContext, err = avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.Hex2Address("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt/2)
	}
	for i := maxValidatorSize; i < maxValidatorSize*2; i++ {
		candidate := common.Hex2Address("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize*2, len(candidateMap))

	// epochTime is not complete, all validators didn't mint enough block at least
	dposContext, err = avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.Hex2Address("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt/2-1)
	}
	for i := maxValidatorSize; i < maxValidatorSize*2; i++ {
		candidate := common.Hex2Address("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, maxValidatorSize, len(candidateMap))

	dposContext, err = avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	// assert.NotNil(t, epochContext.kickoutValidator(testEpoch))
	dposContext.SetValidators([]common.Address{})
	// assert.NotNil(t, epochContext.kickoutValidator(testEpoch))
}

func setTestMintCnt(dposContext *avlmerkle.DposContext, epoch int64, validator common.Address, count int64) {
	for i := int64(0); i < count; i++ {
		updateMintCnt(epoch*epochInterval, epoch*epochInterval+blockInterval, validator, dposContext)
	}
}

func getCandidates(candidateTrie *avlmerkle.Tree) map[common.Address]bool {
	candidateMap := map[common.Address]bool{}
	iter := candidateTrie.NewIterator(nil)
	next := iter.Next()
	for next != nil {

		candidateMap[common.Bytes2Address(next.Value())] = true
	}

	return candidateMap
}

func TestEpochContextTryElect(t *testing.T) {
	db := test.NewMemStorage()
	stateDB := xfsgo.NewStateTree(db, nil)
	dposContext, err := avlmerkle.NewDposContext(db)
	assert.Nil(t, err)
	epochContext := &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	atLeastMintCnt := epochInterval / blockInterval / maxValidatorSize / 2
	testEpoch := int64(1)
	validators := []common.Address{}
	for i := 0; i < maxValidatorSize; i++ {
		validator := common.Hex2Address("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		assert.Nil(t, dposContext.Delegate(validator, validator))
		stateDB.SetBalance(validator, big.NewInt(1))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
	}
	dposContext.BecomeCandidate(common.Hex2Address("more"))
	assert.Nil(t, dposContext.SetValidators(validators))

	// genesisEpoch == parentEpoch do not kickout
	genesis := &xfsgo.BlockHeader{
		Timestamp: uint64(time.Now().Unix()),
	}
	parentTime := big.NewInt(epochInterval - blockInterval)
	parent := &xfsgo.BlockHeader{
		Timestamp: parentTime.Uint64(),
	}
	oldHash := dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err := dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, maxValidatorSize, len(result))
	for _, validator := range result {
		assert.True(t, strings.Contains(validator.B58String(), "addr"))
	}
	assert.HashEqual(t, oldHash, dposContext.EpochTrie().Hash())

	// genesisEpoch != parentEpoch and have none mintCnt do not kickout
	genesisTime := big.NewInt(-epochInterval)
	genesis = &xfsgo.BlockHeader{
		Timestamp: genesisTime.Uint64(),
	}

	parentTimestamp := big.NewInt(epochInterval - blockInterval)
	parent = &xfsgo.BlockHeader{
		Difficulty: big.NewInt(1),
		Timestamp:  parentTimestamp.Uint64(),
	}
	epochContext.TimeStamp = epochInterval
	oldHash = dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err = dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, maxValidatorSize, len(result))
	for _, validator := range result {
		assert.True(t, strings.Contains(validator.B58String(), "addr"))
	}
	assert.HashEqual(t, oldHash, dposContext.EpochTrie().Hash())

	// genesisEpoch != parentEpoch kickout
	genesis = &xfsgo.BlockHeader{
		Timestamp: big.NewInt(0).Uint64(),
	}

	parent = &xfsgo.BlockHeader{
		Timestamp: big.NewInt(epochInterval*2 - blockInterval).Uint64(),
	}

	epochContext.TimeStamp = epochInterval * 2
	oldHash = dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err = dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, safeSize, len(result))
	moreCnt := 0
	for _, validator := range result {
		if strings.Contains(validator.B58String(), "more") {
			moreCnt++
		}
	}
	assert.Equal(t, 1, moreCnt)
	assert.HashEqual(t, oldHash, dposContext.EpochTrie().Hash())

	// parentEpoch == currentEpoch do not elect
	genesis = &xfsgo.BlockHeader{
		Timestamp: big.NewInt(0).Uint64(),
	}
	parent = &xfsgo.BlockHeader{
		Timestamp: big.NewInt(epochInterval).Uint64(),
	}
	epochContext.TimeStamp = epochInterval + blockInterval
	oldHash = dposContext.EpochTrie().Hash()
	assert.Nil(t, epochContext.tryElect(genesis, parent))
	result, err = dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, safeSize, len(result))
	assert.Equal(t, oldHash, dposContext.EpochTrie().Hash())
}
