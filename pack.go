package wtpack

import (
	"errors"
	"strings"
	"unicode"
)

import "bytes"

//import "fmt"

var invalidError = errors.New("Invalid parameter")
var notfoundError = errors.New("Notfound")

type wtpack struct {
	pfmt     *string
	curIdx   int
	repeats  int
	havesize bool
	size     int
	vtype    byte
}

func (p *wtpack) start(pfmt *string) error {
	if len(*pfmt) == 0 {
		*pfmt = "u"
	}

	if (*pfmt)[0] == '@' || (*pfmt)[0] == '<' || (*pfmt)[0] == '>' {
		return invalidError
	}

	if (*pfmt)[0] == '.' {
		p.curIdx = 1
	}

	if p.curIdx == len(*pfmt) {
		return invalidError
	}

	p.pfmt = pfmt

	return nil
}

func (p *wtpack) next() error {
	if p.repeats > 0 {
		p.repeats--
		return nil
	}

pfmt_next:

	if p.curIdx == len(*p.pfmt) {
		return notfoundError
	}

	if unicode.IsDigit(rune((*p.pfmt)[p.curIdx])) {
		p.havesize = true
		p.size = 0

		for ; p.curIdx < len(*p.pfmt) && unicode.IsDigit(rune((*p.pfmt)[p.curIdx])); p.curIdx++ {
			p.size *= 10
			p.size += int((*p.pfmt)[p.curIdx] - '0')
		}

		if p.curIdx == len(*p.pfmt) {
			return invalidError
		}
	} else {
		p.havesize = false
		p.size = 1
	}

	p.vtype = (*p.pfmt)[p.curIdx]

	switch p.vtype {
	case 'S', 'x':
	case 's':
		/* Fixed length strings must be at least 1 byte */
		if p.size < 1 {
			return invalidError
		}
	case 't':
		/* Bitfield sizes must be between 1 and 8 bits */
		if p.size < 1 || p.size > 8 {
			return invalidError
		}
	case 'u', 'U':
		/* Special case for items with a size prefix. */
		if (p.havesize == false) && (p.curIdx != len(*p.pfmt)-1) {
			p.vtype = 'U'
		} else {
			p.vtype = 'u'
		}
	case 'b', 'h', 'i', 'B', 'H', 'I', 'l', 'L', 'q', 'Q', 'r', 'R':
		if p.size == 0 {
			p.curIdx++
			goto pfmt_next
		}

		p.havesize = false
		p.repeats = p.size - 1
	default:
		return invalidError
	}

	p.curIdx++
	return nil
}

func (p *wtpack) reset() {
	p.curIdx = 0
}

func (p *wtpack) pack_size(i interface{}) (int, error) {
	switch p.vtype {
	case 'x':
		return int(p.size), nil
	case 'S', 's':
		v, ok := i.(string)
		if ok == false {
			return 0, invalidError
		}

		if p.vtype == 's' || p.havesize == true {
			return p.size, nil
		} else {
			s := strings.IndexByte(v, 0)
			if s != -1 {
				return s + 1, nil
			}

			return len(v) + 1, nil
		}
	case 'u', 'U':
		v, ok := i.([]byte)
		if ok == false {
			panic(0)
			return 0, invalidError
		}

		s := len(v)
		pad := 0

		switch {
		case p.havesize == true && p.size < s:
			s = p.size
		case p.havesize == true:
			pad = p.size - s
		}

		if p.vtype == 'U' {
			s += vsize_uint(uint64(s + pad))
		}

		return s + pad, nil

	case 'b':
		if _, ok := i.(int8); ok == false {
			panic(0)
			return 0, invalidError
		}

		return 1, nil
	case 'B', 't':
		if _, ok := i.(byte); ok == false {
			panic(0)
			return 0, invalidError
		}

		return 1, nil

	case 'h', 'i', 'l', 'q':
		switch v := i.(type) {
		case int:
			return vsize_int(int64(v)), nil
		case int16:
			return vsize_int(int64(v)), nil
		case int32:
			return vsize_int(int64(v)), nil
		case int64:
			return vsize_int(v), nil
		default:
			panic(0)
			return 0, invalidError
		}

	case 'H', 'I', 'L', 'Q', 'r':
		switch v := i.(type) {
		case uint:
			return vsize_uint(uint64(v)), nil
		case uint16:
			return vsize_uint(uint64(v)), nil
		case uint32:
			return vsize_uint(uint64(v)), nil
		case uint64:
			return vsize_uint(v), nil
		default:
			panic(0)
			return 0, invalidError
		}

	default:
		panic(0)
		return 0, invalidError
	}
}

func (p *wtpack) pack(buf []byte, i interface{}) ([]byte, error) {
	switch p.vtype {
	case 'x':
		for p.size > 0 {
			buf = append(buf, byte(0))
			p.size--
		}
	case 's':
		v, ok := i.(string)
		if !ok {
			return buf[:0], invalidError
		}

		switch {
		case p.size == len(v):
			buf = append(buf, v...)
		case p.size > len(v):
			pad := p.size - len(v)
			buf = append(buf, v...)

			for ; pad != 0; pad-- {
				buf = append(buf, byte(0))
			}
		case p.size < len(v):
			buf = append(buf, v[:p.size]...)
		}
	case 'S':
		v, ok := i.(string)
		if !ok {
			return buf[:0], invalidError
		}

		s := strings.IndexByte(v, 0)
		if s == -1 {
			buf = append(buf, v...)
			buf = append(buf, byte(0))
		} else {
			buf = append(buf, v[:s+1]...)
		}
	case 'u', 'U':
		v, ok := i.([]byte)
		if !ok {
			return buf[:0], invalidError
		}

		s := len(v)
		pad := 0

		switch {
		case p.havesize == true && p.size < s:
			s = p.size
		case p.havesize == true:
			pad = p.size - s
		}

		if p.vtype == 'U' {
			buf = vpack_uint(buf, uint64(s+pad))
		}

		if s > 0 {
			buf = append(buf, v[:s]...)
		}

		for ; pad != 0; pad-- {
			buf = append(buf, byte(0))
		}
	case 'b':
		v, ok := i.(int8)
		if !ok {
			return buf[:0], invalidError
		}

		buf = append(buf, byte(uint8(v)^0x80))
	case 'B', 't':
		v, ok := i.(byte)
		if !ok {
			return buf[:0], invalidError
		}

		buf = append(buf, v)
	case 'h', 'i', 'l', 'q', 'H', 'I', 'L', 'Q', 'r':
		switch v := i.(type) {
		case int:
			buf = vpack_int(buf, int64(v))
		case int16:
			buf = vpack_int(buf, int64(v))
		case int32:
			buf = vpack_int(buf, int64(v))
		case int64:
			buf = vpack_int(buf, v)
		case uint:
			buf = vpack_uint(buf, uint64(v))
		case uint16:
			buf = vpack_uint(buf, uint64(v))
		case uint32:
			buf = vpack_uint(buf, uint64(v))
		case uint64:
			buf = vpack_uint(buf, v)
		default:
			return buf[:0], invalidError
		}
	}

	return buf, nil
}

func (p *wtpack) unpack(buf []byte, bcur *int, bend int, i interface{}) error {
	switch p.vtype {
	case 'x':
		*bcur += p.size
	case 'S', 's':
		var s int
		v, ok := i.(*string)
		if ok == false {
			return invalidError
		}

		if p.vtype == 's' || p.havesize == true {
			s = p.size
			*v = string(buf[*bcur : *bcur+s])
			*bcur += s
		} else {
			s = bytes.IndexByte(buf[*bcur:], 0)
			switch {
			case s == 0:
				*v = ""
				*bcur++
			case s > 0:
				*v = string(buf[*bcur : *bcur+s])
				*bcur += s + 1
			default:
				return invalidError
			}
		}
	case 'u', 'U':
		var s int
		v, ok := i.(*[]byte)
		if ok == false {
			return invalidError
		}

		switch {
		case p.havesize == true:
			s = p.size
		case p.vtype == 'U':
			if su, r := vunpack_uint(buf, bcur, bend); r != nil {
				return r
			} else {
				s = int(su)
			}

		default:
			s = bend - *bcur
		}

		*v = (*v)[:0]
		*v = append(*v, buf[*bcur:*bcur+s]...)
		*bcur += s

	case 'b':
		v, ok := i.(*int8)
		if ok == false {
			return invalidError
		}

		*v = int8(buf[*bcur] ^ 0x80)
		*bcur++

	case 'B', 't':
		v, ok := i.(*uint8)
		if ok == false {
			return invalidError
		}

		*v = buf[*bcur]
		*bcur++

	case 'h', 'i', 'l', 'q':
		if vc, r := vunpack_int(buf, bcur, bend); r != nil {
			return r
		} else {
			switch v := i.(type) {
			case *int:
				*v = int(vc)
			case *int16:
				*v = int16(vc)
			case *int32:
				*v = int32(vc)
			case *int64:
				*v = int64(vc)
			case *uint:
				*v = uint(vc)
			case *uint16:
				*v = uint16(vc)
			case *uint32:
				*v = uint32(vc)
			case *uint64:
				*v = uint64(vc)
			default:
				return invalidError
			}
		}
	case 'H', 'I', 'L', 'Q', 'r':
		if vc, r := vunpack_uint(buf, bcur, bend); r != nil {
			return r
		} else {
			switch v := i.(type) {
			case *int:
				*v = int(vc)
			case *int16:
				*v = int16(vc)
			case *int32:
				*v = int32(vc)
			case *int64:
				*v = int64(vc)
			case *uint:
				*v = uint(vc)
			case *uint16:
				*v = uint16(vc)
			case *uint32:
				*v = uint32(vc)
			case *uint64:
				*v = uint64(vc)
			default:
				return invalidError
			}
		}
	default:
		return invalidError
	}

	return nil
}

func PackFormated(pfmt string, buf []byte, a ...interface{}) ([]byte, error) {
	var res error
	var cidx int

	buf = buf[:0]

	pcnt := len(a)

	wtp := new(wtpack)
	if res = wtp.start(&pfmt); res != nil {
		return nil, res
	}

	res = wtp.next()
	for res == nil {
		if wtp.vtype == 'x' {
			buf, res = wtp.pack(buf, byte(0))
			res = wtp.next()
			continue
		}

		if cidx == pcnt {
			res = invalidError
			buf = buf[:0]
			break
		}

		if buf, res = wtp.pack(buf, a[cidx]); res != nil {
			break
		}

		res = wtp.next()
		cidx++
	}

	if res != nil && res != notfoundError {
		return buf, res
	}

	return buf, nil
}

func UnPackFormated(pfmt string, buf []byte, a ...interface{}) error {
	var res error
	var cidx int
	var bcur int

	pcnt := len(a)
	bend := len(buf)

	if bend == 0 {
		return invalidError
	}

	wtp := new(wtpack)
	if res = wtp.start(&pfmt); res != nil {
		return invalidError
	}

	res = wtp.next()
	for res == nil {
		if wtp.vtype == 'x' {
			res = wtp.unpack(buf, &bcur, bend, byte(0))
			res = wtp.next()
			continue
		}

		if pcnt == 0 {
			res = invalidError
			break
		}

		if res = wtp.unpack(buf, &bcur, bend, a[cidx]); res == nil {
			res = wtp.next()
			cidx++
			pcnt--
		}
	}

	if res != nil && res != notfoundError {
		return res
	}

	return nil
}

func PackInterface(a ...interface{}) []byte {
	var buf []byte
	if a == nil || len(a) == 0 {
		return nil
	}

	lastArg := len(a) - 1

	for i, arg := range a {
		switch v := arg.(type) {
		case string:
			s := strings.IndexByte(v, 0)
			if s != -1 {
				buf = append(buf, v[:s+1]...)
			} else {
				buf = append(buf, v...)
				buf = append(buf, byte(0))
			}
		case []byte:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			buf = append(buf, v...)
		case int8:
			buf = append(buf, byte(uint8(v)^0x80))
		case byte:
			buf = append(buf, v)
		case int:
			buf = vpack_int(buf, int64(v))
		case int16:
			buf = vpack_int(buf, int64(v))
		case int32:
			buf = vpack_int(buf, int64(v))
		case int64:
			buf = vpack_int(buf, v)
		case uint:
			buf = vpack_uint(buf, uint64(v))
		case uint16:
			buf = vpack_uint(buf, uint64(v))
		case uint32:
			buf = vpack_uint(buf, uint64(v))
		case uint64:
			buf = vpack_uint(buf, v)

		case []int:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_int(buf, int64(va))
			}
		case []int16:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_int(buf, int64(va))
			}
		case []int32:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_int(buf, int64(va))
			}
		case []int64:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_int(buf, va)
			}

		case []uint:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_uint(buf, uint64(va))
			}
		case []uint16:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_uint(buf, uint64(va))
			}
		case []uint32:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_uint(buf, uint64(va))
			}
		case []uint64:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				buf = vpack_uint(buf, va)
			}

		case []string:
			if i != lastArg {
				buf = vpack_uint(buf, uint64(len(v)))
			}
			for _, va := range v {
				s := strings.IndexByte(va, 0)
				if s != -1 {
					buf = append(buf, va[:s+1]...)
				} else {
					buf = append(buf, va...)
					buf = append(buf, byte(0))
				}
			}

		default:
			return nil
		}
	}

	return buf
}

func UnPackInterface(buf []byte, a ...interface{}) error {
	var bcur int

	if len(buf) == 0 || a == nil || len(a) == 0 {
		return nil
	}

	lastArg := len(a) - 1
	bend := len(buf)

	for i, arg := range a {
		switch v := arg.(type) {
		case *string:
			s := bytes.IndexByte(buf[bcur:], 0)
			switch {
			case s == 0:
				*v = ""
				bcur++
			case s > 0:
				*v = string(buf[bcur : bcur+s])
				bcur += s + 1
			default:
				return invalidError
			}
		case *[]byte:
			var s int

			if i != lastArg {
				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
			} else {
				s = bend - bcur
			}

			*v = buf[bcur : bcur+s]
			bcur += s
		case *int8:
			*v = int8(buf[bcur] ^ 0x80)
			bcur++
		case *byte:
			*v = buf[bcur]
			bcur++
		case *int:
			if vc, r := vunpack_int(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = int(vc)
			}
		case *int16:
			if vc, r := vunpack_int(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = int16(vc)
			}
		case *int32:
			if vc, r := vunpack_int(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = int32(vc)
			}
		case *int64:
			if vc, r := vunpack_int(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = vc
			}
		case *uint:
			if vc, r := vunpack_uint(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = uint(vc)
			}
		case *uint16:
			if vc, r := vunpack_uint(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = uint16(vc)
			}
		case *uint32:
			if vc, r := vunpack_uint(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = uint32(vc)
			}
		case *uint64:
			if vc, r := vunpack_uint(buf, &bcur, bend); r != nil {
				return r
			} else {
				*v = vc
			}

		case *[]int:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, int(va))
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, int(va))
					}
				}
			}
		case *[]int16:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, int16(va))
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, int16(va))
					}
				}
			}
		case *[]int32:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, int32(va))
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, int32(va))
					}
				}
			}
		case *[]int64:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, va)
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_int(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, va)
					}
				}
			}

		case *[]uint:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, uint(va))
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, uint(va))
					}
				}
			}
		case *[]uint16:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, uint16(va))
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, uint16(va))
					}
				}
			}
		case *[]uint32:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, uint32(va))
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, uint32(va))
					}
				}
			}
		case *[]uint64:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, va)
					}
				}
			} else {
				for bend-bcur > 0 {
					if va, r := vunpack_uint(buf, &bcur, bend); r != nil {
						return r
					} else {
						*v = append(*v, va)
					}
				}
			}

		case *[]string:
			if i != lastArg {
				var s int

				if su, r := vunpack_uint(buf, &bcur, bend); r != nil {
					return r
				} else {
					s = int(su)
				}
				for ; s > 0; s-- {
					ss := bytes.IndexByte(buf[bcur:], 0)
					switch {
					case ss == 0:
						*v = append(*v, "")
						bcur++
					case ss > 0:
						*v = append(*v, string(buf[bcur:bcur+ss]))
						bcur += ss + 1
					default:
						return invalidError
					}
				}
			} else {
				for bend-bcur > 0 {
					ss := bytes.IndexByte(buf[bcur:], 0)
					switch {
					case ss == 0:
						*v = append(*v, "")
						bcur++
					case ss > 0:
						*v = append(*v, string(buf[bcur:bcur+ss]))
						bcur += ss + 1
					default:
						return invalidError
					}
				}
			}

		default:
			return invalidError
		}
	}

	return nil
}
