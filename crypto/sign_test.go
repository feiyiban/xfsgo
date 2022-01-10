package crypto

import (
	"encoding/hex"
	"testing"
	"xfsgo/common"
	"xfsgo/common/ahash"
)

func TestECDSASign(t *testing.T) {
	data := "hello"
	key := MustGenPrvKey()
	datahash := ahash.SHA256([]byte(data))
	signed, err := ECDSASign(datahash, key)
	if err != nil {
		t.Fatal(err)
	}
	if verified := ECDSAVerifySignature(key.PublicKey, datahash, signed); !verified {
		t.Fatal("check sign failed")
	}
}

func TestCreateAddress(t *testing.T) {
	fromAddressText := "aJTobAyvdXeEGW7DHA1Yqc6PaVa2apHdX"
	fromAddress := common.StrB58ToAddress(fromAddressText)
	fromAddressHashBytes := ahash.SHA256(fromAddress.Bytes())
	fromAddressHash := common.Bytes2Hash(fromAddressHashBytes)

	fromAddressHashHex := hex.EncodeToString(fromAddressHash[:])
	t.Logf("got hex=%s", fromAddressHashHex)
	gotAddress := CreateAddress(fromAddressHash, 1)
	t.Logf("got address=%s", gotAddress.B58String())
}
