package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"xfsgo/common"
	"xfsgo/common/ahash"
	"xfsgo/crypto/secp256k1"
	"xfsgo/rlp"

	"golang.org/x/crypto/sha3"
)

const defaultKeyPackType = uint8(1)
const DefaultKeyPackVersion = uint8(1)
const DigestLength = 32

//SignatureLength indicates the byte length required to carry a signature with recovery id.
const SignatureLength = 64 + 1 // 64 bytes ECDSA signature + 1 byte recovery id

var (
	secp256k1N, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	secp256k1halfN = new(big.Int).Div(secp256k1N, big.NewInt(2))
)

func GenPrvKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(secp256k1.S256(), rand.Reader)
}

func MustGenPrvKey() *ecdsa.PrivateKey {
	key, err := GenPrvKey()
	if err != nil {
		print(err)
	}
	return key
}

func PubKeyEncode(p ecdsa.PublicKey) []byte {
	if p.Curve == nil || p.X == nil || p.Y == nil {
		return nil
	}
	xbs := p.X.Bytes()
	ybs := p.Y.Bytes()
	buf := make([]byte, len(xbs)+len(ybs))
	copy(buf, append(xbs, ybs...))
	return buf
}

func Checksum(payload []byte) []byte {
	first := ahash.SHA256(payload)
	second := ahash.SHA256(first)
	return second[:common.AddrCheckSumLen]
}

func VerifyAddress(addr common.Address) bool {
	want := Checksum(addr.Payload())
	got := addr.Checksum()
	return bytes.Equal(want, got)
}

func DefaultPubKey2Addr(p ecdsa.PublicKey) common.Address {
	return PubKey2Addr(common.DefaultAddressVersion, p)
}

func PubKey2Addr(version uint8, p ecdsa.PublicKey) common.Address {
	pubEnc := PubKeyEncode(p)
	pubHash256 := ahash.SHA256(pubEnc)
	pubHash := ahash.Ripemd160(pubHash256)
	payload := append([]byte{version}, pubHash...)
	cs := Checksum(payload)
	full := append(payload, cs...)
	return common.Bytes2Address(full)
}

func EncodePrivateKey(version uint8, key *ecdsa.PrivateKey) []byte {
	dbytes := key.D.Bytes()

	curve := secp256k1.S256()
	curveOrder := curve.Params().N
	privateKey := make([]byte, (curveOrder.BitLen()+7)/8)
	for len(dbytes) > len(privateKey) {
		if dbytes[0] != 0 {
			return nil
		}
		dbytes = dbytes[1:]
	}
	copy(privateKey[len(privateKey)-len(dbytes):], dbytes)

	buf := append([]byte{version, defaultKeyPackType}, privateKey...)
	return buf
}

func DefaultEncodePrivateKey(key *ecdsa.PrivateKey) []byte {
	return EncodePrivateKey(DefaultKeyPackVersion, key)
}

func DecodePrivateKey(bs []byte) (uint8, *ecdsa.PrivateKey, error) {
	if len(bs) <= 2 {
		return 0, nil, errors.New("unknown private key version")
	}
	version := bs[0]
	keytype := bs[1]
	payload := bs[2:]
	priv := new(ecdsa.PrivateKey)
	if keytype == 1 {
		k := new(big.Int).SetBytes(payload)
		curve := secp256k1.S256()
		curveOrder := curve.Params().N
		if k.Cmp(curveOrder) >= 0 {
			return 0, nil, errors.New("invalid elliptic curve private key value")
		}
		priv.Curve = curve
		priv.D = k
		privateKey := make([]byte, (curveOrder.BitLen()+7)/8)
		for len(payload) > len(privateKey) {
			if payload[0] != 0 {
				return 0, nil, errors.New("invalid private key length")
			}
			payload = payload[1:]
		}

		// Some private keys remove all leading zeros, this is also invalid
		// according to [SEC1] but since OpenSSL used to do this, we ignore
		// this too.
		copy(privateKey[len(privateKey)-len(payload):], payload)
		priv.X, priv.Y = curve.ScalarBaseMult(privateKey)
	} else {
		return 0, nil, errors.New("unknown private key encrypt type")
	}

	return version, priv, nil
}

func ByteHash256(raw []byte) common.Hash {
	h := ahash.SHA256(raw)
	return common.Bytes2Hash(h)
}

// KeccakState wraps sha3.state. In addition to the usual hash methods, it also supports
// Read to get a variable amount of data from the hash state. Read is faster than Sum
// because it doesn't copy the internal state, but also modifies the internal state.
type KeccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

// NewKeccakState creates a new KeccakState
func NewKeccakState() KeccakState {
	return sha3.NewLegacyKeccak256().(KeccakState)
}

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	b := make([]byte, 32)
	d := NewKeccakState()
	for _, b := range data {
		d.Write(b)
	}
	_, _ = d.Read(b)
	return b
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h common.Hash) {
	d := NewKeccakState()
	for _, b := range data {
		d.Write(b)
	}
	_, _ = d.Read(h[:])
	return h
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func ValidateSignatureValues(v byte, r, s *big.Int, homestead bool) bool {
	if r.Cmp(common.Big1) < 0 || s.Cmp(common.Big1) < 0 {
		return false
	}
	// reject upper range of s values (ECDSA malleability)
	// see discussion in secp256k1/libsecp256k1/include/secp256k1.h
	if homestead && s.Cmp(secp256k1halfN) > 0 {
		return false
	}
	// Frontier: allow s to be in full N range
	return r.Cmp(secp256k1N) < 0 && s.Cmp(secp256k1N) < 0 && (v == 0 || v == 1)
}

// CreateAddress2 creates an ethereum address given the address bytes, initial
// contract code hash and a salt.
func CreateAddress2(b common.Address, salt [32]byte, inithash []byte) common.Address {
	return common.Bytes2Address(Keccak256([]byte{0xff}, b.Bytes(), salt[:], inithash)[12:])
}

// CreateAddress creates an ethereum address given the bytes and the nonce
func CreateAddress(b common.Address, nonce uint64) common.Address {
	data, _ := rlp.EncodeToBytes([]interface{}{b, nonce})
	return common.Bytes2Address(Keccak256(data)[12:])
}

// ToECDSA creates a private key with the given D value.
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	return toECDSA(d, true)
}

// toECDSA creates a private key with the given D value. The strict parameter
// controls whether the key's length should be enforced at the curve size or
// it can also accept legacy encodings (0 prefixes).
func toECDSA(d []byte, strict bool) (*ecdsa.PrivateKey, error) {
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = S256()
	if strict && 8*len(d) != priv.Params().BitSize {
		return nil, fmt.Errorf("invalid length, need %d bits", priv.Params().BitSize)
	}
	priv.D = new(big.Int).SetBytes(d)

	// The priv.D must < N
	if priv.D.Cmp(secp256k1N) >= 0 {
		return nil, fmt.Errorf("invalid private key, >=N")
	}
	// The priv.D must not be zero or negative.
	if priv.D.Sign() <= 0 {
		return nil, fmt.Errorf("invalid private key, zero or negative")
	}

	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(d)
	if priv.PublicKey.X == nil {
		return nil, errors.New("invalid private key")
	}
	return priv, nil
}
