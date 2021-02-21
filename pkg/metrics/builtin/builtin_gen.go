package builtin

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Providers) DecodeMsg(dc *msgp.Reader) (err error) {
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
		case "items":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "Items")
				return
			}
			if z.Items == nil {
				z.Items = make(map[string]int32, zb0002)
			} else if len(z.Items) > 0 {
				for key := range z.Items {
					delete(z.Items, key)
				}
			}
			for zb0002 > 0 {
				zb0002--
				var za0001 string
				var za0002 int32
				za0001, err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Items")
					return
				}
				za0002, err = dc.ReadInt32()
				if err != nil {
					err = msgp.WrapError(err, "Items", za0001)
					return
				}
				z.Items[za0001] = za0002
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
func (z *Providers) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 1
	// write "items"
	err = en.Append(0x81, 0xa5, 0x69, 0x74, 0x65, 0x6d, 0x73)
	if err != nil {
		return
	}
	err = en.WriteMapHeader(uint32(len(z.Items)))
	if err != nil {
		err = msgp.WrapError(err, "Items")
		return
	}
	for za0001, za0002 := range z.Items {
		err = en.WriteString(za0001)
		if err != nil {
			err = msgp.WrapError(err, "Items")
			return
		}
		err = en.WriteInt32(za0002)
		if err != nil {
			err = msgp.WrapError(err, "Items", za0001)
			return
		}
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Providers) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 1
	// string "items"
	o = append(o, 0x81, 0xa5, 0x69, 0x74, 0x65, 0x6d, 0x73)
	o = msgp.AppendMapHeader(o, uint32(len(z.Items)))
	for za0001, za0002 := range z.Items {
		o = msgp.AppendString(o, za0001)
		o = msgp.AppendInt32(o, za0002)
	}
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Providers) UnmarshalMsg(bts []byte) (o []byte, err error) {
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
		case "items":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Items")
				return
			}
			if z.Items == nil {
				z.Items = make(map[string]int32, zb0002)
			} else if len(z.Items) > 0 {
				for key := range z.Items {
					delete(z.Items, key)
				}
			}
			for zb0002 > 0 {
				var za0001 string
				var za0002 int32
				zb0002--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Items")
					return
				}
				za0002, bts, err = msgp.ReadInt32Bytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Items", za0001)
					return
				}
				z.Items[za0001] = za0002
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
func (z *Providers) Msgsize() (s int) {
	s = 1 + 6 + msgp.MapHeaderSize
	if z.Items != nil {
		for za0001, za0002 := range z.Items {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + msgp.Int32Size
		}
	}
	return
}