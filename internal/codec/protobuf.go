package codec

import (
	"fmt"
	"math"
	"strconv"

	"google.golang.org/protobuf/encoding/protowire"
)

func consumeProtoTag(body *[]byte) (protowire.Number, protowire.Type, bool, error) {
	if len(*body) == 0 {
		return 0, 0, false, nil
	}
	number, typ, n := protowire.ConsumeTag(*body)
	if n < 0 {
		return 0, 0, false, fmt.Errorf("invalid tag: %w", protowire.ParseError(n))
	}
	*body = (*body)[n:]
	return number, typ, true, nil
}

func consumeProtoVarint(body *[]byte, typ protowire.Type) (uint64, error) {
	if typ != protowire.VarintType {
		return 0, fmt.Errorf("want varint wire type, got %d", typ)
	}
	value, n := protowire.ConsumeVarint(*body)
	if n < 0 {
		return 0, protowire.ParseError(n)
	}
	*body = (*body)[n:]
	return value, nil
}

func consumeProtoDouble(body *[]byte, typ protowire.Type) (float64, error) {
	if typ != protowire.Fixed64Type {
		return 0, fmt.Errorf("want fixed64 wire type, got %d", typ)
	}
	bits, n := protowire.ConsumeFixed64(*body)
	if n < 0 {
		return 0, protowire.ParseError(n)
	}
	*body = (*body)[n:]
	return math.Float64frombits(bits), nil
}

func consumeProtoBytes(body *[]byte, typ protowire.Type) ([]byte, error) {
	if typ != protowire.BytesType {
		return nil, fmt.Errorf("want bytes wire type, got %d", typ)
	}
	value, n := protowire.ConsumeBytes(*body)
	if n < 0 {
		return nil, protowire.ParseError(n)
	}
	*body = (*body)[n:]
	return value, nil
}

func skipProtoField(body *[]byte, number protowire.Number, typ protowire.Type) error {
	n := protowire.ConsumeFieldValue(number, typ, *body)
	if n < 0 {
		return protowire.ParseError(n)
	}
	*body = (*body)[n:]
	return nil
}

func appendProtoVarint(dst []byte, number protowire.Number, value uint64) []byte {
	dst = protowire.AppendTag(dst, number, protowire.VarintType)
	return protowire.AppendVarint(dst, value)
}

func appendProtoDouble(dst []byte, number protowire.Number, value float64) []byte {
	dst = protowire.AppendTag(dst, number, protowire.Fixed64Type)
	return protowire.AppendFixed64(dst, math.Float64bits(value))
}

func appendProtoString(dst []byte, number protowire.Number, value string) []byte {
	dst = protowire.AppendTag(dst, number, protowire.BytesType)
	return protowire.AppendString(dst, value)
}

func appendProtoMessage(dst []byte, number protowire.Number, value []byte) []byte {
	dst = protowire.AppendTag(dst, number, protowire.BytesType)
	return protowire.AppendBytes(dst, value)
}

func formatProtoDouble(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}
