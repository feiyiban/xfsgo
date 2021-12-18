package avlmerkle

import (
	"testing"

	"xfsgo/common"
	"xfsgo/test"

	"github.com/stretchr/testify/assert"
)

func TestDposContextSnapshot(t *testing.T) {
	db := test.NewMemStorage()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)

	snapshot := dposContext.Snapshot()
	assert.Equal(t, dposContext.Root(), snapshot.Root())
	assert.NotEqual(t, dposContext, snapshot)

	// change dposContext
	assert.Nil(t, dposContext.BecomeCandidate(common.Hex2Address("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6c")))
	assert.NotEqual(t, dposContext.Root(), snapshot.Root())

	// revert snapshot
	dposContext.RevertToSnapShot(snapshot)
	assert.Equal(t, dposContext.Root(), snapshot.Root())
	assert.NotEqual(t, dposContext, snapshot)
}

func TestDposContextBecomeCandidate(t *testing.T) {
	candidates := []common.Address{
		common.Hex2Address("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e"),
		common.Hex2Address("0xa60a3886b552ff9992cfcd208ec1152079e046c2"),
		common.Hex2Address("0x4e080e49f62694554871e669aeb4ebe17c4a9670"),
	}
	db := test.NewMemStorage()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)
	for _, candidate := range candidates {
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}

	candidateMap := map[common.Address]bool{}
	candidateIter := dposContext.candidateTrie.NewIterator(nil)
	next := candidateIter.Next()
	for next != nil {
		candidateMap[common.Bytes2Address(next.Value())] = true
	}
	assert.Equal(t, len(candidates), len(candidateMap))
	for _, candidate := range candidates {
		assert.True(t, candidateMap[candidate])
	}
}

func TestDposContextKickoutCandidate(t *testing.T) {
	candidates := []common.Address{
		common.Hex2Address("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e"),
		common.Hex2Address("0xa60a3886b552ff9992cfcd208ec1152079e046c2"),
		common.Hex2Address("0x4e080e49f62694554871e669aeb4ebe17c4a9670"),
	}
	db := test.NewMemStorage()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)
	for _, candidate := range candidates {
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
		assert.Nil(t, dposContext.Delegate(candidate, candidate))
	}

	kickIdx := 1
	assert.Nil(t, dposContext.KickoutCandidate(candidates[kickIdx]))
	candidateMap := map[common.Address]bool{}
	candidateIter := dposContext.candidateTrie.NewIterator(nil)
	next := candidateIter.Next()
	for next != nil {
		candidateMap[common.Bytes2Address(next.Value())] = true
	}
	voteIter := dposContext.voteTrie.NewIterator(nil)
	voteMap := map[common.Address]bool{}
	voteIternext := voteIter.Next()
	for next != nil {
		voteMap[common.Bytes2Address(voteIternext.Value())] = true
	}
	for i, candidate := range candidates {
		delegateIterNext := dposContext.delegateTrie.NewIterator(candidate.Bytes())
		if i == kickIdx {
			assert.False(t, delegateIterNext == nil)
			assert.False(t, candidateMap[candidate])
			assert.False(t, voteMap[candidate])
			continue
		}
		assert.True(t, delegateIterNext == nil)
		assert.True(t, candidateMap[candidate])
		assert.True(t, voteMap[candidate])
	}
}

func TestDposContextDelegateAndUnDelegate(t *testing.T) {
	candidate := common.Hex2Address("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e")
	newCandidate := common.Hex2Address("0xa60a3886b552ff9992cfcd208ec1152079e046c2")
	delegator := common.Hex2Address("0x4e080e49f62694554871e669aeb4ebe17c4a9670")
	db := test.NewMemStorage()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)
	assert.Nil(t, dposContext.BecomeCandidate(candidate))
	assert.Nil(t, dposContext.BecomeCandidate(newCandidate))

	// delegator delegate to not exist candidate
	candidateIter := dposContext.candidateTrie.NewIterator(nil)
	candidateMap := map[string]bool{}
	candidateIterNext := candidateIter.Next()
	for candidateIterNext != nil {
		candidateMap[string(candidateIterNext.Value())] = true
	}
	assert.NotNil(t, dposContext.Delegate(delegator, common.Hex2Address("0xab")))

	// delegator delegate to old candidate
	assert.Nil(t, dposContext.Delegate(delegator, candidate))

	delegateIter := dposContext.delegateTrie.NewIterator(candidate.Bytes())
	delegateIterNext := delegateIter.Next()
	if delegateIterNext != nil {
		assert.Equal(t, append(delegatePrefix, append(candidate.Bytes(), delegator.Bytes()...)...), delegateIter.Key)
		assert.Equal(t, delegator, common.Bytes2Address(delegateIterNext.Value()))
	}
	voteIter := dposContext.voteTrie.NewIterator(nil)
	voteIterNext := voteIter.Next()
	if voteIterNext != nil {
		assert.Equal(t, append(votePrefix, delegator.Bytes()...), voteIterNext.Key())
		assert.Equal(t, candidate, common.Bytes2Address(voteIterNext.Value()))
	}

	// delegator delegate to new candidate
	assert.Nil(t, dposContext.Delegate(delegator, newCandidate))
	delegateIter = dposContext.delegateTrie.NewIterator(candidate.Bytes())
	// assert.False(t, delegateIter.Next())
	delegateIter = dposContext.delegateTrie.NewIterator(newCandidate.Bytes())
	delegateIterNextP := delegateIter.Next()
	if delegateIterNextP != nil {
		assert.Equal(t, append(delegatePrefix, append(newCandidate.Bytes(), delegator.Bytes()...)...), delegateIterNextP.Key())
		assert.Equal(t, delegator, common.Bytes2Address(delegateIterNextP.Value()))
	}
	voteIter = dposContext.voteTrie.NewIterator(nil)
	voteIterNextP := voteIter.Next()
	if voteIterNextP != nil {
		assert.Equal(t, append(votePrefix, delegator.Bytes()...), voteIterNextP.Key())
		assert.Equal(t, newCandidate, common.Bytes2Address(voteIterNextP.Value()))
	}

	// delegator undelegate to not exist candidate
	assert.NotNil(t, dposContext.UnDelegate(common.Hex2Address("0x00"), candidate))

	// delegator undelegate to old candidate
	assert.NotNil(t, dposContext.UnDelegate(delegator, candidate))

	// delegator undelegate to new candidate
	assert.Nil(t, dposContext.UnDelegate(delegator, newCandidate))
	delegateIter = dposContext.delegateTrie.NewIterator(newCandidate.Bytes())
	// assert.False(t, delegateIter.Next())

	voteIter = dposContext.voteTrie.NewIterator(nil)
	// assert.False(t, voteIter.Next())
}

func TestDposContextValidators(t *testing.T) {
	validators := []common.Address{
		common.Hex2Address("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e"),
		common.Hex2Address("0xa60a3886b552ff9992cfcd208ec1152079e046c2"),
		common.Hex2Address("0x4e080e49f62694554871e669aeb4ebe17c4a9670"),
	}

	db := test.NewMemStorage()
	dposContext, err := NewDposContext(db)
	assert.Nil(t, err)

	assert.Nil(t, dposContext.SetValidators(validators))

	result, err := dposContext.GetValidators()
	assert.Nil(t, err)
	assert.Equal(t, len(validators), len(result))
	validatorMap := map[common.Address]bool{}
	for _, validator := range validators {
		validatorMap[validator] = true
	}
	for _, validator := range result {
		assert.True(t, validatorMap[validator])
	}
}
