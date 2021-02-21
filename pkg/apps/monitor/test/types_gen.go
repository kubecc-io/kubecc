package test

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *TestKey1) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "counter":
			z.Counter, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Counter")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z TestKey1) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 1
	// write "counter"
	err = en.Append(0x81, 0xa7, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x65, 0x72)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Counter)
	if err != nil {
		err = msgp.WrapError(err, "Counter")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z TestKey1) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 1
	// string "counter"
	o = append(o, 0x81, 0xa7, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x65, 0x72)
	o = msgp.AppendInt(o, z.Counter)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TestKey1) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "counter":
			z.Counter, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Counter")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z TestKey1) Msgsize() (s int) {
	s = 1 + 8 + msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *TestKey2) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "value":
			z.Value, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Value")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z TestKey2) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 1
	// write "value"
	err = en.Append(0x81, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Value)
	if err != nil {
		err = msgp.WrapError(err, "Value")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z TestKey2) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 1
	// string "value"
	o = append(o, 0x81, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65)
	o = msgp.AppendString(o, z.Value)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *TestKey2) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "value":
			z.Value, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Value")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z TestKey2) Msgsize() (s int) {
	s = 1 + 6 + msgp.StringPrefixSize + len(z.Value)
	return
}
