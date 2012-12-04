package etf

import (
	"bytes"
	"github.com/ftrvxmtrx/testingo"
	"io"
	"math"
	r "reflect"
	"testing"
)

func testWrite(
	t *testingo.TT,
	fi, pi, v interface{},
	shouldSize uint,
	shouldError bool,
	args ...interface{}) {

	f := func(w io.Writer, data interface{}) interface{} {
		return r.ValueOf(fi).Call([]r.Value{
			r.ValueOf(w),
			r.ValueOf(data),
		})[0].Interface()
	}

	p := func(b []byte) (ret interface{}, size uint, err interface{}) {
		result := r.ValueOf(pi).Call([]r.Value{r.ValueOf(b)})
		ret = result[0].Interface()
		size = result[1].Interface().(uint)
		err = result[2].Interface()
		return
	}

	var result interface{}
	var resultSize uint
	var err interface{}

	w := new(bytes.Buffer)
	w.Reset()
	err = f(w, v)

	if !shouldError {
		t.AssertEq(nil, err, args...)
		t.AssertEq(shouldSize, uint(w.Len()), "encode", args)
		result, resultSize, err = p(w.Bytes())
		t.AssertEq(nil, err, args...)
		t.AssertEq(v, result, args...)
		t.AssertEq(shouldSize, resultSize, "decode", args)
	} else {
		t.AssertNotEq(nil, err, args...)
		switch err.(type) {
		case EncodeError:
		default:
			t.Fatalf("error is not EncodeError, but %T (%#v)", err, args)
		}
	}
}

func Test_writeAtom(t0 *testing.T) {
	t := testingo.T(t0)

	testWriteAtom := func(v string, headerSize uint, shouldError bool, args ...interface{}) {
		testWrite(t, writeAtom, parseAtom, Atom(v), headerSize+uint(len(v)), shouldError, args...)
	}

	testWriteAtom(string(bytes.Repeat([]byte{'a'}, math.MaxUint8+0)), 2, false, "255 $a")
	testWriteAtom(string(bytes.Repeat([]byte{'a'}, math.MaxUint8+1)), 3, false, "256 $a")
	testWriteAtom("", 2, false, "'' (empty atom)")
	testWriteAtom(string(bytes.Repeat([]byte{'a'}, math.MaxUint16+0)), 3, false, "65535 $a")
	testWriteAtom(string(bytes.Repeat([]byte{'a'}, math.MaxUint16+1)), 3, true, "65536 $a")
}

func Test_writeBinary(t0 *testing.T) {
	t := testingo.T(t0)

	testWriteBinary := func(bytes []byte, headerSize uint, shouldError bool, args ...interface{}) {
		testWrite(t, writeBinary, parseBinary, bytes, headerSize+uint(len(bytes)), shouldError, args...)
	}

	testWriteBinary([]byte{}, 5, false, "empty binary")
	testWriteBinary(bytes.Repeat([]byte{1}, 64), 5, false, "65535 bytes binary")
}

func Test_writeBool(t0 *testing.T) {
	t := testingo.T(t0)

	testWriteBool := func(b bool, totalSize uint, args ...interface{}) {
		testWrite(t, writeBool, parseBool, b, totalSize, false, args...)
	}

	testWriteBool(true, 6, "true")
	testWriteBool(false, 7, "false")
}

func Test_writeFloat64(t0 *testing.T) {
	t := testingo.T(t0)

	testWriteFloat64 := func(f float64) {
		testWrite(t, writeFloat64, parseFloat64, f, 9, false, f)
	}

	testWriteFloat64(0.0)
	testWriteFloat64(math.SmallestNonzeroFloat64)
	testWriteFloat64(math.MaxFloat64)
}

func Test_writeInt64_and_BigInt(t0 *testing.T) {
	t := testingo.T(t0)

	testWriteInt64 := func(x int64, totalSize uint, shouldError bool, args ...interface{}) {
		testWrite(t, writeInt64, parseInt64, x, totalSize, shouldError, args...)
	}

	testWriteInt64(0, 2, false, "0")
	testWriteInt64(-1, 5, false, "0")
	testWriteInt64(math.MaxUint8+0, 2, false, "255")
	testWriteInt64(math.MaxUint8+1, 5, false, "256")
	testWriteInt64(math.MaxInt32+0, 5, false, "0x7fffffff")
	testWriteInt64(math.MaxInt32+1, 7, false, "0x80000000")
	testWriteInt64(math.MinInt32+0, 5, false, "-0x80000000")
	testWriteInt64(math.MinInt32-1, 7, false, "-0x80000001")
}

func Test_writeString(t0 *testing.T) {
	t := testingo.T(t0)

	testWriteString := func(v string, headerSize uint, shouldError bool, args ...interface{}) {
		testWrite(t, writeString, parseString, v, headerSize+uint(len(v)), shouldError, args...)
	}

	testWriteString(string(bytes.Repeat([]byte{'a'}, math.MaxUint16+0)), 3, false, "65535 $a")
	testWriteString("", 3, false, `"" (empty string)`)
	testWriteString(string(bytes.Repeat([]byte{'a'}, math.MaxUint16+1)), 3, true, "65536 $a")
}