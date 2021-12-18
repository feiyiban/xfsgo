package consensus

import (
	"xfsgo"
	"xfsgo/avlmerkle"
	"xfsgo/common"
)

// Engine is an algorithm agnostic consensus engine.
type Engine interface {
	// Author retrieves the Ethereum address of the account that minted the given
	// block, which may be different from the header's coinbase if a consensus
	// engine is based on signatures.
	Author(header *xfsgo.BlockHeader) (common.Address, error)

	// Coinbase return the benefits owner of the given block
	Coinbase(header *xfsgo.BlockHeader) (common.Address, error)

	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader, seal bool) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(chain xfsgo.IBlockChain, headers []*xfsgo.BlockHeader, seals []bool) (chan<- struct{}, <-chan error)

	// VerifyUncles verifies that the given block's uncles conform to the consensus
	// rules of a given engine.
	// VerifyUncles(chain xfsgo.IBlockChain, block *xfsgo.Block) error

	// VerifySeal checks whether the crypto seal on a header is valid according to
	// the consensus rules of the given engine.
	VerifySeal(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader) error

	// Prepare initializes the consensus fields of a block header according to the
	// rules of a particular engine. The changes are executed inline.
	Prepare(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader) error

	// Finalize runs any post-transaction state modifications (e.g. block rewards)
	// and assembles the final block.
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	Finalize(chain xfsgo.IBlockChain, header *xfsgo.BlockHeader, state *xfsgo.StateTree, txs []*xfsgo.Transaction, receipts []*xfsgo.Receipt, dposContext *avlmerkle.DposContext) (*xfsgo.Block, error)

	// Seal generates a new block for the given input block with the local miner's
	// seal place on top.
	Seal(chain xfsgo.IBlockChain, block *xfsgo.Block, stop <-chan struct{}) (*xfsgo.Block, error)

	// APIs returns the RPC APIs this consensus engine provides.
	APIs(chain xfsgo.IBlockChain) error
}

// PoW is a consensus engine based on proof-of-work.
type PoW interface {
	Engine

	// Hashrate returns the current mining hashrate of a PoW consensus engine.
	Hashrate() float64
}
