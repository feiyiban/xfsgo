package common

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type BlocksMap map[string]interface{}

func (block BlocksMap) MapMerge() map[string]interface{} {

	result := make(map[string]interface{}, 1)
	blockheader := block
	_, ok := block["header"].(map[string]interface{})
	if ok {
		blockheader = block["header"].(map[string]interface{})
	}

	result["version"] = blockheader["version"]
	result["height"] = blockheader["height"]
	result["hash_prev_block"] = blockheader["hash_prev_block"]
	result["hash"] = blockheader["hash"]
	result["timestamp"] = time.Unix(int64(blockheader["timestamp"].(float64)), 0).UTC().Format(time.RFC3339)
	result["state_root"] = blockheader["state_root"]
	result["transactions_root"] = blockheader["transactions_root"]
	result["receipts_root"] = blockheader["receipts_root"]
	bitsStr := strconv.FormatInt(int64(blockheader["bits"].(float64)), 10)
	result["bits"] = bitsStr
	result["nonce"] = blockheader["nonce"]
	result["coinbase"] = blockheader["coinbase"]

	gas := blockheader["gas_limit"].(float64)
	gasUesd := blockheader["gas_used"].(float64)
	result["gas_limit"] = gas
	result["gas_used"] = gasUesd
	return result
}

func Marshal(info map[string]interface{}, sortIndex []string, isIndent bool) (string, error) {

	if len(info) != len(sortIndex) {
		return "", errors.New("inconsistent array length")
	}

	var jsonBuf strings.Builder
	jsonBuf.WriteString("{")
	for i := 0; i < len(sortIndex); i++ {
		k := sortIndex[i]
		jsonBuf.WriteString("\"" + k + "\":")
		var content string
		mydata := reflect.ValueOf(info).MapIndex(reflect.ValueOf(k))

		switch reflect.ValueOf(mydata.Interface()).Kind() {
		case reflect.String:
			content = "\"" + reflect.ValueOf(mydata.Interface()).String() + "\""
		case reflect.Float64:
			v := reflect.ValueOf(mydata.Interface())
			strings := strconv.FormatInt(int64(v.Float()), 10)
			content = strings
		default:
			content = "null"
		}
		if i < len(sortIndex)-1 {
			jsonBuf.WriteString(content + ",")
		} else {
			jsonBuf.WriteString(content)
		}
	}
	jsonBuf.WriteString("}")
	if !isIndent {
		return jsonBuf.String(), nil
	}
	var retBuf bytes.Buffer
	err := json.Indent(&retBuf, []byte(jsonBuf.String()), "", "    ")

	if err != nil {
		return "", err
	}
	_, _ = retBuf.WriteTo(os.Stdout)
	return retBuf.String(), nil

}

func Marshals(info []BlocksMap, sortIndex []string, isIndent bool) (string, error) {

	var jsonBuf bytes.Buffer
	jsonBuf.WriteString("[")
	for index, item := range info {
		jsonBuf.WriteString("{")
		for i := 0; i < len(sortIndex); i++ {
			k := sortIndex[i]
			jsonBuf.WriteString("\"" + k + "\":")
			var content string
			mydata := reflect.ValueOf(item).MapIndex(reflect.ValueOf(k))
			switch reflect.ValueOf(mydata.Interface()).Kind() {
			case reflect.String:
				content = "\"" + reflect.ValueOf(mydata.Interface()).String() + "\""
			case reflect.Float64:
				v := reflect.ValueOf(mydata.Interface())
				strings := strconv.FormatInt(int64(v.Float()), 10)
				content = strings
			default:
				content = "null"
			}
			if i < len(sortIndex)-1 {
				jsonBuf.WriteString(content + ",")
			} else {
				jsonBuf.WriteString(content)
			}
		}
		if index < len(info)-1 {
			jsonBuf.WriteString("},")
		} else {
			jsonBuf.WriteString("}")
		}

	}
	jsonBuf.WriteString("]")
	if !isIndent {
		return jsonBuf.String(), nil
	}
	var retBuf bytes.Buffer
	err := json.Indent(&retBuf, jsonBuf.Bytes(), "", "\t")

	if err != nil {
		return "", err
	}
	_, _ = retBuf.WriteTo(os.Stdout)
	return retBuf.String(), nil

}

func MarshalIndent(val interface{}) ([]byte, error) {
	return json.MarshalIndent(val, "", "    ")
}

func Struct2Bytes(iter interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.BigEndian, iter)
	if err != nil {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

func Bytes2Struct(buf []byte) unsafe.Pointer {
	return unsafe.Pointer(
		(*reflect.SliceHeader)(unsafe.Pointer(&buf)).Data,
	)
}

func Int2Byte(data int) (ret []byte) {
	var len uintptr = unsafe.Sizeof(data)
	ret = make([]byte, len)
	var tmp int = 0xff
	var index uint = 0
	for index = 0; index < uint(len); index++ {
		ret[index] = byte((tmp << (index * 8) & data) >> (index * 8))
	}
	return ret
}

func Byte2Int(data []byte) int {
	var ret int = 0
	var len int = len(data)
	var i uint = 0
	for i = 0; i < uint(len); i++ {
		ret = ret | (int(data[i]) << (i * 8))
	}
	return ret
}
