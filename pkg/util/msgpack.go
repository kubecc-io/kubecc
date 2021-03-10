package util

import (
	"bytes"

	"github.com/tinylib/msgp/msgp"
)

func EncodeMsgp(e msgp.Encodable) []byte {
	buf := new(bytes.Buffer)
	w := msgp.NewWriter(buf)
	if err := e.EncodeMsg(w); err != nil {
		panic(err)
	}
	if err := w.Flush(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func DecodeMsgp(buf []byte, into msgp.Decodable) error {
	reader := msgp.NewReader(bytes.NewReader(buf))
	return into.DecodeMsg(reader)
}
