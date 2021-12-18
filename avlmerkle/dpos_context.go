package avlmerkle

import (
	"bytes"
	"errors"
	"fmt"

	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/common/rawencode"
	"xfsgo/storage/badger"
)

type DposContext struct {
	epochTrie     *Tree
	delegateTrie  *Tree
	voteTrie      *Tree
	candidateTrie *Tree
	mintCntTrie   *Tree
	db            badger.IStorage
}

var (
	epochPrefix     = []byte("epoch:")
	delegatePrefix  = []byte("delegate:")
	votePrefix      = []byte("vote:")
	candidatePrefix = []byte("candidate:")
	mintCntPrefix   = []byte("mintCnt:")
)

func NewEpochTrie(root common.Hash, db badger.IStorage) (*Tree, error) {
	return NewTreeN(db, root[:], []byte(epochPrefix))
	// return trie.NewTrieWithPrefix(root, epochPrefix, db)
}

func NewDelegateTrie(root common.Hash, db badger.IStorage) (*Tree, error) {
	return NewTreeN(db, root[:], []byte(delegatePrefix))
	// 	return trie.NewTrieWithPrefix(root, delegatePrefix, db)
}

func NewVoteTrie(root common.Hash, db badger.IStorage) (*Tree, error) {
	return NewTreeN(db, root[:], []byte(votePrefix))
	// 	return trie.NewTrieWithPrefix(root, votePrefix, db)
}

func NewCandidateTrie(root common.Hash, db badger.IStorage) (*Tree, error) {
	return NewTreeN(db, root[:], []byte(candidatePrefix))
	// 	return trie.NewTrieWithPrefix(root, candidatePrefix, db)
}

func NewMintCntTrie(root common.Hash, db badger.IStorage) (*Tree, error) {
	return NewTreeN(db, root[:], []byte(mintCntPrefix))
	// return trie.NewTrieWithPrefix(root, mintCntPrefix, db)
}

func NewDposContext(db badger.IStorage) (*DposContext, error) {
	epochTrie, err := NewEpochTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}
	delegateTrie, err := NewDelegateTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}
	voteTrie, err := NewVoteTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}
	candidateTrie, err := NewCandidateTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}
	mintCntTrie, err := NewMintCntTrie(common.Hash{}, db)
	if err != nil {
		return nil, err
	}
	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		mintCntTrie:   mintCntTrie,
		db:            db,
	}, nil
}

func NewDposContextFromProto(db badger.IStorage, ctxProto *DposContextProto) (*DposContext, error) {
	epochTrie, err := NewEpochTrie(ctxProto.EpochHash, db)
	if err != nil {
		return nil, err
	}
	delegateTrie, err := NewDelegateTrie(ctxProto.DelegateHash, db)
	if err != nil {
		return nil, err
	}
	voteTrie, err := NewVoteTrie(ctxProto.VoteHash, db)
	if err != nil {
		return nil, err
	}
	candidateTrie, err := NewCandidateTrie(ctxProto.CandidateHash, db)
	if err != nil {
		return nil, err
	}
	mintCntTrie, err := NewMintCntTrie(ctxProto.MintCntHash, db)
	if err != nil {
		return nil, err
	}
	return &DposContext{
		epochTrie:     epochTrie,
		delegateTrie:  delegateTrie,
		voteTrie:      voteTrie,
		candidateTrie: candidateTrie,
		mintCntTrie:   mintCntTrie,
		db:            db,
	}, nil
}

func (d *DposContext) Copy() *DposContext {
	epochTrie := *d.epochTrie
	delegateTrie := *d.delegateTrie
	voteTrie := *d.voteTrie
	candidateTrie := *d.candidateTrie
	mintCntTrie := *d.mintCntTrie
	return &DposContext{
		epochTrie:     &epochTrie,
		delegateTrie:  &delegateTrie,
		voteTrie:      &voteTrie,
		candidateTrie: &candidateTrie,
		mintCntTrie:   &mintCntTrie,
	}
}

// bert
func (d *DposContext) Root() common.Hash {
	var hw bytes.Buffer
	if bs, err := rawencode.Encode(d.epochTrie.Hash()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(d.delegateTrie.Hash()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(d.candidateTrie.Hash()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(d.voteTrie.Hash()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(d.mintCntTrie.Hash()); err != nil {
		hw.Write(bs)
	}

	return common.Bytes2Hash(ahash.SHA256(hw.Bytes()))
}

func (d *DposContext) Snapshot() *DposContext {
	return d.Copy()
}

func (d *DposContext) RevertToSnapShot(snapshot *DposContext) {
	d.epochTrie = snapshot.epochTrie
	d.delegateTrie = snapshot.delegateTrie
	d.candidateTrie = snapshot.candidateTrie
	d.voteTrie = snapshot.voteTrie
	d.mintCntTrie = snapshot.mintCntTrie
}

func (d *DposContext) FromProto(dcp *DposContextProto) error {
	var err error
	d.epochTrie, err = NewEpochTrie(dcp.EpochHash, d.db)
	if err != nil {
		return err
	}
	d.delegateTrie, err = NewDelegateTrie(dcp.DelegateHash, d.db)
	if err != nil {
		return err
	}
	d.candidateTrie, err = NewCandidateTrie(dcp.CandidateHash, d.db)
	if err != nil {
		return err
	}
	d.voteTrie, err = NewVoteTrie(dcp.VoteHash, d.db)
	if err != nil {
		return err
	}
	d.mintCntTrie, err = NewMintCntTrie(dcp.MintCntHash, d.db)
	return err
}

type DposContextProto struct {
	EpochHash     common.Hash `json:"epochRoot"        gencodec:"required"`
	DelegateHash  common.Hash `json:"delegateRoot"     gencodec:"required"`
	CandidateHash common.Hash `json:"candidateRoot"    gencodec:"required"`
	VoteHash      common.Hash `json:"voteRoot"         gencodec:"required"`
	MintCntHash   common.Hash `json:"mintCntRoot"      gencodec:"required"`
}

func (d *DposContext) ToProto() *DposContextProto {
	return &DposContextProto{
		EpochHash:     d.epochTrie.Hash(),
		DelegateHash:  d.delegateTrie.Hash(),
		CandidateHash: d.candidateTrie.Hash(),
		VoteHash:      d.voteTrie.Hash(),
		MintCntHash:   d.mintCntTrie.Hash(),
	}
}

func (p *DposContextProto) Root() common.Hash {
	var hw bytes.Buffer

	if bs, err := rawencode.Encode(p.EpochHash.Bytes()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(p.DelegateHash.Bytes()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(p.CandidateHash.Bytes()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(p.VoteHash.Bytes()); err != nil {
		hw.Write(bs)
	}
	if bs, err := rawencode.Encode(p.MintCntHash.Bytes()); err != nil {
		hw.Write(bs)
	}

	return common.Bytes2Hash(hw.Bytes())
}

func (d *DposContext) KickoutCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	// d.candidateTrie.
	err := d.candidateTrie.Remove(candidate)
	if err != nil {
		return err
	}

	iter := d.delegateTrie.NewIterator(candidate)
	iterNext := iter.Next()
	for iterNext != nil {
		delegator := iterNext.Value()
		key := append(candidate, delegator...)
		if err := d.delegateTrie.Remove(key); err != nil {
			return err
		}
		v, ok := d.voteTrie.Get(delegator)
		if !ok {
			return errors.New("missingNodeError")
		}
		if err == nil && bytes.Equal(v, candidate) {
			err = d.voteTrie.Remove(delegator)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func (d *DposContext) BecomeCandidate(candidateAddr common.Address) error {
	candidate := candidateAddr.Bytes()
	return d.candidateTrie.Update(candidate, candidate)
}

func (d *DposContext) Delegate(delegatorAddr, candidateAddr common.Address) error {
	delegator, candidate := delegatorAddr.Bytes(), candidateAddr.Bytes()

	// the candidate must be candidate
	candidateInTrie, ok := d.candidateTrie.Get(candidate)
	if !ok {
		return errors.New("get not null")
	}
	if candidateInTrie == nil {
		return errors.New("invalid candidate to delegate")
	}

	// delete old candidate if exists
	oldCandidate, ok := d.voteTrie.Get(delegator)
	if ok {
		return errors.New("delete old candidate if exists")
	}
	if oldCandidate != nil {
		d.delegateTrie.Remove(append(oldCandidate, delegator...))
	}

	if err := d.delegateTrie.Update(append(candidate, delegator...), delegator); err != nil {
		return err
	}
	return d.voteTrie.Update(delegator, candidate)
}

func (d *DposContext) UnDelegate(delegatorAddr, candidateAddr common.Address) error {
	delegator, candidate := delegatorAddr.Bytes(), candidateAddr.Bytes()

	// the candidate must be candidate
	candidateInTrie, ok := d.candidateTrie.Get(candidate)
	if !ok {
		return errors.New("the candidate must be candidate")
	}
	if candidateInTrie == nil {
		return errors.New("invalid candidate to undelegate")
	}

	oldCandidate, ok := d.voteTrie.Get(delegator)
	if !ok {
		return errors.New("the candidate must be candidate")
	}

	if !bytes.Equal(candidate, oldCandidate) {
		return errors.New("mismatch candidate to undelegate")
	}

	if err := d.delegateTrie.Remove(append(candidate, delegator...)); err != nil {
		return err
	}
	return d.voteTrie.Remove(delegator)
}

func (d *DposContext) CommitTo() (*DposContextProto, error) {
	if err := d.epochTrie.Commit(); err != nil {
		return nil, err
	}

	if err := d.delegateTrie.Commit(); err != nil {
		return nil, err
	}

	if err := d.voteTrie.Commit(); err != nil {
		return nil, err
	}

	if err := d.candidateTrie.Commit(); err != nil {
		return nil, err
	}

	if err := d.mintCntTrie.Commit(); err != nil {
		return nil, err
	}
	return &DposContextProto{
		EpochHash:     d.epochTrie.Hash(),
		DelegateHash:  d.delegateTrie.Hash(),
		VoteHash:      d.voteTrie.Hash(),
		CandidateHash: d.candidateTrie.Hash(),
		MintCntHash:   d.mintCntTrie.Hash(),
	}, nil
}

func (d *DposContext) CandidateTrie() *Tree          { return d.candidateTrie }
func (d *DposContext) DelegateTrie() *Tree           { return d.delegateTrie }
func (d *DposContext) VoteTrie() *Tree               { return d.voteTrie }
func (d *DposContext) EpochTrie() *Tree              { return d.epochTrie }
func (d *DposContext) MintCntTrie() *Tree            { return d.mintCntTrie }
func (d *DposContext) DB() badger.IStorage           { return d.db }
func (dc *DposContext) SetEpoch(epoch *Tree)         { dc.epochTrie = epoch }
func (dc *DposContext) SetDelegate(delegate *Tree)   { dc.delegateTrie = delegate }
func (dc *DposContext) SetVote(vote *Tree)           { dc.voteTrie = vote }
func (dc *DposContext) SetCandidate(candidate *Tree) { dc.candidateTrie = candidate }
func (dc *DposContext) SetMintCnt(mintCnt *Tree)     { dc.mintCntTrie = mintCnt }

func (dc *DposContext) GetValidators() ([]common.Address, error) {
	var validators []common.Address
	key := []byte("validator")
	validatorsRLP, ok := dc.epochTrie.Get(key)
	if !ok {
		return nil, errors.New("get key not data")
	}

	if err := rawencode.Decode(validatorsRLP, &validators); err != nil {
		return nil, fmt.Errorf("failed to decode validators: %s", err)
	}
	return validators, nil
}

func (dc *DposContext) SetValidators(validators []common.Address) error {
	key := []byte("validator")

	validatorsRLP, err := rawencode.Encode(validators)
	if err != nil {
		return fmt.Errorf("failed to encode validators to rlp bytes: %s", err)
	}
	dc.epochTrie.Update(key, validatorsRLP)
	return nil
}
