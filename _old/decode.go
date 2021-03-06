// Package etf provides an API to decode Erlang terms into Go
// data structures and vice versa.
package etf

import (
	"encoding/binary"
	"errors"
	"fmt"
	read "github.com/goerlang/etf/read"
	t "github.com/goerlang/etf/types"
	"io"
	"math/big"
	"reflect"
)

var (
	atomType     = reflect.ValueOf(t.Atom("")).Type()
	ErrBadFormat = errors.New("etf: bad format")
)

// Decode unmarshals a value and stores it to a variable pointed by ptr.
func Decode(r io.Reader, ptr interface{}) (err error) {
	b := make([]byte, 1)
	_, err = io.ReadFull(r, b)
	if err == nil {
		if b[0] != t.EtVersion {
			err = fmt.Errorf("version %d not supported", b[0])
			return
		}

		p := reflect.ValueOf(ptr)
		err = decode(r, p)
	}

	return
}

func decode(r io.Reader, ptr reflect.Value) (err error) {
	v := ptr.Elem()

	switch v.Kind() {
	case reflect.Bool:
		var result bool
		if result, err = read.Bool(r); err == nil {
			v.SetBool(result)
		}

	case reflect.Int,
		reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var result int64
		if result, err = read.Int(r); err != nil {
			break
		}
		if v.OverflowInt(result) {
			err = fmt.Errorf("%v overflows %T", result, v.Type())
		} else {
			v.SetInt(result)
		}

	case reflect.Uint, reflect.Uintptr,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var result uint64
		if result, err = read.Uint(r); err != nil {
			break
		}
		if v.OverflowUint(result) {
			err = fmt.Errorf("%v overflows %T", result, v.Type())
		} else {
			v.SetUint(result)
		}

	case reflect.Float32, reflect.Float64:
		var result float64
		if result, err = read.Float(r); err != nil {
			break
		}
		if v.OverflowFloat(result) {
			err = fmt.Errorf("%v overflows %T", result, v.Type())
		} else {
			v.SetFloat(result)
		}

	case reflect.Interface:
		// FIXME

	case reflect.Map:
		// FIXME

	case reflect.Ptr:
		err = decodeSpecial(r, v)

	case reflect.String:
		if v.Type() == atomType {
			var result t.Atom
			if result, err = read.Atom(r); err == nil {
				v.Set(reflect.ValueOf(result))
			}
		} else {
			var result string
			if result, err = read.String(r); err == nil {
				v.Set(reflect.ValueOf(result))
			}
		}

	case reflect.Array:
		err = decodeArray(r, v)

	case reflect.Slice:
		err = decodeSlice(r, v)

	case reflect.Struct:
		err = decodeStruct(r, v)

	default:
		err = fmt.Errorf("unsupported type %v", v.Type())
	}

	return
}

func decodeArray(r io.Reader, v reflect.Value) (err error) {
	length := v.Len()

	switch v.Type().Elem().Kind() {
	case reflect.Uint8:
		var result []byte
		if result, err = read.Binary(r); err == nil {
			if length == len(result) {
				for i := range result {
					v.Index(i).SetUint(uint64(result[i]))
				}
			} else {
				err = fmt.Errorf("%v overflows %T", result, v.Type())
			}
		}

	default:
		err = decodeList(r, v)
	}

	return
}

func decodeList(r io.Reader, v reflect.Value) (err error) {
	b := make([]byte, 1)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		switch b[0] {
		case t.EttList:
			// $lLLLL…$j
			var listLen uint32
			if err = binary.Read(r, binary.BigEndian, &listLen); err != nil {
				return
			}

			slice := reflect.MakeSlice(v.Type(), int(listLen), int(listLen))
			for i := uint32(0); i < listLen; i++ {
				if err = decode(r, slice.Index(int(i)).Addr()); err != nil {
					return
				}
			}

			_, err = io.ReadFull(r, b)
			if err == nil && b[0] != t.EttNil {
				err = read.ErrImproperList
			} else {
				v.Set(slice)
			}

		case t.EttNil:
			// empty slice -- do not touch it
			return nil
		}

	default:
		err = fmt.Errorf("unsupported type %v", v.Type())
	}

	return
}

func decodeSlice(r io.Reader, v reflect.Value) (err error) {
	switch v.Interface().(type) {
	case []byte:
		var result []byte
		if result, err = read.Binary(r); err == nil {
			v.SetBytes(result)
		}

	default:
		err = decodeList(r, v)
	}

	return
}

func decodeSpecial(r io.Reader, v reflect.Value) (err error) {
	switch v.Interface().(type) {
	case *big.Int:
		var result *big.Int
		if result, err = read.BigInt(r); err == nil {
			v.Set(reflect.ValueOf(result))
		}

	default:
		err = fmt.Errorf("unsupported type %v", v.Type())
	}

	return
}

func decodeStruct(r io.Reader, v reflect.Value) (err error) {
	var arity int
	b := make([]byte, 1)
	_, err = io.ReadFull(r, b)
	if err != nil {
		return
	}

	switch b[0] {
	case t.EttSmallTuple:
		// $hA…
		var a uint8
		if err = binary.Read(r, binary.BigEndian, &a); err == nil {
			arity = int(a)
		}

	case t.EttLargeTuple:
		// $iAAAA…
		var a uint32
		if err = binary.Read(r, binary.BigEndian, &a); err == nil {
			arity = int(a)
		}

	default:
		err = &read.ErrTypeDiffer{
			b[0],
			[]byte{
				t.EttSmallTuple,
				t.EttLargeTuple,
			},
		}
	}

	if err != nil {
		return
	}

	var fieldsSet int
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.CanSet() {
			err = decode(r, f.Addr())
			fieldsSet++

			if err != nil {
				break
			}
		}
	}

	if arity != fieldsSet {
		err = fmt.Errorf(
			"different number of fields (%d, should be %d)",
			v.NumField(),
			arity,
		)
		return
	}

	return
}
