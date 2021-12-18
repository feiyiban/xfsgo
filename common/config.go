package common

import (
	"fmt"
	"math/big"
)

var ( // Maximum size extra data may be after Genesis.
	DifficultyBoundDivisor = big.NewInt(2048)   // The bound divisor of the difficulty, used in the update calculations.
	GenesisDifficulty      = big.NewInt(131072) // Difficulty of the Genesis block.
	MinimumDifficulty      = big.NewInt(131072)
	DurationLimit          = big.NewInt(13)
)

type ChainConfig struct {
	ChainId *big.Int `json:"chainId"` // Chain id identifies the current chain and is used for replay protection

	HomesteadBlock *big.Int `json:"homesteadBlock,omitempty"` // Homestead switch block (nil = no fork, 0 = already homestead)

	ByzantiumBlock *big.Int `json:"byzantiumBlock,omitempty"` // Byzantium switch block (nil = no fork, 0 = already on byzantium)

	Dpos *DposConfig `json:"dpos,omitempty"`
}

// IsHomestead returns whether num is either equal to the homestead block or greater.
func (c *ChainConfig) IsHomestead(num *big.Int) bool {
	return isForked(c.HomesteadBlock, num)
}

func isForked(s, head *big.Int) bool {
	if s == nil || head == nil {
		return false
	}
	return s.Cmp(head) <= 0
}

// IsDAO returns whether num is either equal to the DAO fork block or greater.
// func (c *ChainConfig) IsDAOFork(num *big.Int) bool {
// 	return isForked(c.DAOForkBlock, num)
// }

// func (c *ChainConfig) IsEIP150(num *big.Int) bool {
// 	return isForked(c.EIP150Block, num)
// }

// func (c *ChainConfig) IsEIP155(num *big.Int) bool {
// 	return isForked(c.EIP155Block, num)
// }

// func (c *ChainConfig) IsEIP158(num *big.Int) bool {
// 	return isForked(c.EIP158Block, num)
// }

func (c *ChainConfig) IsByzantium(num *big.Int) bool {
	return isForked(c.ByzantiumBlock, num)
}

// DposConfig is the consensus engine configs for delegated proof-of-stake based sealing.
type DposConfig struct {
	Validators []Address `json:"validators"` // Genesis validator list
}

// String implements the stringer interface, returning the consensus engine details.
func (d *DposConfig) String() string {
	return "dpos"
}

// String implements the fmt.Stringer interface.
func (c *ChainConfig) String() string {
	return fmt.Sprintf("{ChainID: %v Homestead: %v Byzantium: %v Engine: %v}",
		c.ChainId,
		c.HomesteadBlock,
		// c.DAOForkBlock,
		// c.DAOForkSupport,
		// c.EIP150Block,
		// c.EIP155Block,
		// c.EIP158Block,
		c.ByzantiumBlock,
		c.Dpos,
	)
}
