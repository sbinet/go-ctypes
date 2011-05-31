package ctypes

/*
 #include <string.h>
 #include <stdlib.h>
*/
import "C"

import (
	//"fmt"
	"os"
	"reflect"
	"runtime"
	"unsafe"
)

// Type is the representation of a C type.
type Type interface {

	// Name returns the type's name within its package.
	// It returns an empty string for unnamed types
	Name() string

	// PkgPath returns the type's package path.
	// The package path is a full package import path like "container/vector".
	// PkgPath returns an empty string for unnamed types.
	PkgPath() string

	// Size returns the number of bytes needed to store
	// a value of the given type; it is analogous to unsafe.Sizeof.
	Size() uintptr

	// String returns a string representation of the type.
	// The string representation may use shortened package names
	// (e.g., vector instead of "container/vector") and is not
	// guaranteed to be unique among types.  To test for equality,
	// compare the Types directly.
	String() string

	// Kind returns the specific kind of this type.
	Kind() Kind

	// Elem returns a type's element type.
	// It panics if the type's Kind is not Array, Chan, Map, Ptr, or Slice.
	Elem() Type

	// Field returns a struct type's i'th field.
	// It panics if the type's Kind is not Struct.
	// It panics if i is not in the range [0, NumField()).
	Field(i int) StructField

	// Len returns an array type's length.
	// It panics if the type's Kind is not Array.
	Len() int

	// NumField returns a struct type's field count.
	// It panics if the type's Kind is not Struct.
	NumField() int

	// GoType returns the original reflect.Type which is being shadowed
	GoType() reflect.Type
}

// A Kind represents the specific kind of type that a Type represents.
// The zero Kind is not a valid kind.
type Kind reflect.Kind

func (k Kind) String() string {
	return reflect.Kind(k).String()
}

const (
	Invalid Kind = Kind(reflect.Invalid)
	Bool         = Kind(reflect.Bool)
	Int          = Kind(reflect.Int)
	Int8         = Kind(reflect.Int8)
	Int16        = Kind(reflect.Int16)
	Int32        = Kind(reflect.Int32)
	Int64        = Kind(reflect.Int64)
	Uint         = Kind(reflect.Uint)
	Uint8        = Kind(reflect.Uint8)
	Uint16       = Kind(reflect.Uint16)
	Uint32       = Kind(reflect.Uint32)
	Uint64       = Kind(reflect.Uint64)
	Uintptr      = Kind(reflect.Uintptr)
	Float32      = Kind(reflect.Float32)
	Float64      = Kind(reflect.Float64)
	Complex64    = Kind(reflect.Complex64)
	Complex128   = Kind(reflect.Complex128)
	Array        = Kind(reflect.Array)
	//Chan        
	//Func
	//Interface
	//Map // <-- FIXME? can we implement this ?
	Ptr           = Kind(reflect.Ptr)
	Slice         = Kind(reflect.Slice)
	String        = Kind(reflect.String)
	Struct        = Kind(reflect.Struct)
	UnsafePointer = Kind(reflect.UnsafePointer)
)

type StructField struct {
	PkgPath   string // empty for uppercase Name
	Name      string
	Type      Type
	Tag       string
	Offset    uintptr
	Index     []int
	Anonymous bool
}

type cstring *C.char

type Value struct {
	b        []byte          // the C value for that Value
	t        Type            // the C type of that Value
	idx      int             // the cursor index in the byte buffer of the C-value
	cstrings map[int]cstring // a pool of C-string we own. index is the offset in the Value.b buffer
}

func follow_ptr(v reflect.Value) reflect.Value {
	rv := v
	for {
		switch rv.Kind() {
		case reflect.Ptr:
			rv = rv.Elem()
		default:
			return rv
		}
	}
	return rv
}

// ValueOf returns the ctypes.Value corresponding to the Go-value v
func ValueOf(v interface{}) *Value {
	rv := reflect.ValueOf(v)
	//fmt.Printf("valueof--> %v\n",rv)
	rv = follow_ptr(rv)
	//fmt.Printf("valueof==> %v\n",rv)
	ct := gotype_to_ctype(rv.Type())
	return New(ct)
}

func New(t Type) *Value {
	if t == nil {
		panic("ctypes: New(nil)")
	}
	v := &Value{
		b:        make([]byte, t.Size()),
		t:        t,
		idx:      0,
		cstrings: make(map[int]cstring),
	}

	runtime.SetFinalizer(v, (*Value).Reset)
	return v
}

func (v *Value) Reset() {
	v.idx = 0
	for i := range v.cstrings {
		C.free(unsafe.Pointer(v.cstrings[i]))
	}
	v.cstrings = make(map[int]cstring)

	for i := range v.b {
		v.b[i] = byte(0)
	}
}

func (v *Value) Buffer() []byte {
	return v.b
}

func (v *Value) Type() Type {
	return v.t
}

// C type for a float-complex
type floatcomplex struct {
	real float32
	imag float32
}

var (
	g_complex64  reflect.Type = reflect.TypeOf(floatcomplex{})
	g_complex128 reflect.Type = reflect.TypeOf(doublecomplex{})
)

// C type for a double-complex
type doublecomplex struct {
	real float64
	imag float64
}

// get the C type corresponding to a Go value
func TypeOf(v interface{}) Type {
	rt := reflect.TypeOf(v)
	return gotype_to_ctype(rt)
}

// get the C type corresponding to a Go type
func gotype_to_ctype(t reflect.Type) Type {
	switch t.Kind() {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return &common_type{t}

	case reflect.Complex64:
		return new_cstruct(g_complex64)

	case reflect.Complex128:
		return new_cstruct(g_complex128)

	case reflect.Ptr:
		return &common_type{t}

	case reflect.Array:
		return &common_type{t}

	case reflect.Slice:
		return &vlarray_type{common_type{t}}

	case reflect.String:
		return &cstring_type{common_type{t}}

	case reflect.Struct:
		return new_cstruct(t)

	case reflect.UnsafePointer:
		return &common_type{t}

	default:
		panic("not handled type")
	}
	return nil
}

type ctype struct {
	gotype reflect.Type // the Go type this C-type shadows
}

// a type whose Go type exactly matches the C one
type common_type struct {
	reflect.Type
}

func (t *common_type) Kind() Kind {
	return Kind(t.Type.Kind())
}

func (t *common_type) Elem() Type {
	return gotype_to_ctype(t.Type.Elem())
}

func (t *common_type) Field(i int) (c StructField) {
	f := t.Type.Field(i)
	c = StructField{
		PkgPath: f.PkgPath,
		Name:    f.Name,
		Type:    gotype_to_ctype(f.Type),
		Tag:     f.Tag,
		// FIXME?: this should be corrected for vlarrays/cstrings
		Offset: f.Offset,
		// FIXME?: ditto
		Index:     f.Index,
		Anonymous: f.Anonymous,
	}
	return
}

func (t *common_type) GoType() reflect.Type {
	return t.Type
}

type vlarray_type struct {
	common_type "vlarray"
}

func (t *vlarray_type) Size() uintptr {
	sz := reflect.TypeOf(uintptr(0)).Size() + reflect.TypeOf(int(0)).Size()
	return sz
}

type cstring_type struct {
	common_type "cstring"
}

func (t *cstring_type) Size() uintptr {
	ptr_sz := reflect.TypeOf(uintptr(0)).Size()
	//elem_sz := reflect.TypeOf(byte(0)).Size()
	//nelems_sz := reflect.TypeOf(int(0)).Size()
	return ptr_sz // + nelems_sz
}


type cstruct_type struct {
	common_type "cstruct"
	fields      map[string]Type
}

func new_cstruct(t reflect.Type) *cstruct_type {
	c := &cstruct_type{
		common_type: common_type{t},
		fields:      make(map[string]Type)}

	nfields := t.NumField()
	for i := 0; i < nfields; i++ {
		f := t.Field(i)
		c.fields[f.Name] = gotype_to_ctype(f.Type)
	}
	return c
}

func (t *cstruct_type) Field(i int) (c StructField) {
	f := t.Type.Field(i)
	c = StructField{
		PkgPath: f.PkgPath,
		Name:    f.Name,
		Type:    gotype_to_ctype(f.Type),
		Tag:     f.Tag,
		// FIXME?: this should be corrected for vlarrays/cstrings
		Offset: f.Offset,
		// FIXME?: ditto
		Index:     f.Index,
		Anonymous: f.Anonymous,
	}
	return
}

func (t *cstruct_type) Size() uintptr {
	sz := uintptr(0)
	for _, v := range t.fields {
		// FIXME: alignment ?
		sz += v.Size()
	}
	return sz
}


const (
	sz_bool = unsafe.Sizeof(bool(true))

	sz_int   = unsafe.Sizeof(int(0))
	sz_int8  = unsafe.Sizeof(int8(0))
	sz_int16 = unsafe.Sizeof(int16(0))
	sz_int32 = unsafe.Sizeof(int32(0))
	sz_int64 = unsafe.Sizeof(int64(0))

	sz_uint   = unsafe.Sizeof(uint(0))
	sz_uint8  = unsafe.Sizeof(uint8(0))
	sz_uint16 = unsafe.Sizeof(uint16(0))
	sz_uint32 = unsafe.Sizeof(uint32(0))
	sz_uint64 = unsafe.Sizeof(uint64(0))

	sz_uintptr = unsafe.Sizeof(uintptr(0))

	sz_float32 = unsafe.Sizeof(float32(0))
	sz_float64 = unsafe.Sizeof(float64(0))

	sz_complex64  = unsafe.Sizeof(complex(float32(0), float32(0)))
	sz_complex128 = unsafe.Sizeof(complex(float64(0), float64(0)))
)

// An Encoder is bound to a particular reflect.Type and knows how to
// convert a Go value into a ctypes.Value
type Encoder interface {
	Encode(v interface{}) (*Value, os.Error)
}

type ctype_encoder struct {
	v *Value // the C-value in which we encode
}

// Create a new encoder bound to the C-value v
func NewEncoder(v *Value) Encoder {
	return &ctype_encoder{v: v}
}

// Encode a Go value into a ctypes.Value
func (e *ctype_encoder) Encode(v interface{}) (*Value, os.Error) {
	rv := follow_ptr(reflect.ValueOf(v))
	rt := rv.Type()
	if rt != e.v.Type().GoType() {
		return nil, os.NewError("cannot encode this type [" + rt.String() + "]")
	}

	e.v.Reset()
	encode_value(e.v, rv)
	return e.v, nil
}

type enc_op func(v *Value, p unsafe.Pointer)
var enc_op_table []enc_op

func encode_noop(v *Value, p unsafe.Pointer) {
	panic("noop!")
}

func encode_bool(v *Value, p unsafe.Pointer) {
	src := (*bool)(p)
	dst := (*bool)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_bool
}

func encode_int(v *Value, p unsafe.Pointer) {
	src := (*int)(p)
	dst := (*int)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_int
}

func encode_int8(v *Value, p unsafe.Pointer) {
	src := (*int8)(p)
	dst := (*int8)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_int8
}

func encode_int16(v *Value, p unsafe.Pointer) {
	src := (*int16)(p)
	dst := (*int16)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_int16
}

func encode_int32(v *Value, p unsafe.Pointer) {
	src := (*int32)(p)
	dst := (*int32)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_int32
}

func encode_int64(v *Value, p unsafe.Pointer) {
	src := (*int64)(p)
	dst := (*int64)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_int64
}

func encode_uint(v *Value, p unsafe.Pointer) {
	src := (*uint)(p)
	dst := (*uint)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_uint
}

func encode_uint8(v *Value, p unsafe.Pointer) {
	src := (*uint8)(p)
	dst := (*uint8)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_uint8
}

func encode_uint16(v *Value, p unsafe.Pointer) {
	src := (*uint16)(p)
	dst := (*uint16)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_uint16
}

func encode_uint32(v *Value, p unsafe.Pointer) {
	src := (*uint32)(p)
	dst := (*uint32)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_uint32
}

func encode_uint64(v *Value, p unsafe.Pointer) {
	src := (*uint64)(p)
	dst := (*uint64)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_uint64
}

func encode_uintptr(v *Value, p unsafe.Pointer) {
	src := (*uintptr)(p)
	dst := (*uintptr)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_uintptr
}

func encode_float32(v *Value, p unsafe.Pointer) {
	src := (*float32)(p)
	dst := (*float32)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_float32
}

func encode_float64(v *Value, p unsafe.Pointer) {
	src := (*float64)(p)
	dst := (*float64)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_float64
}

func encode_complex64(v *Value, p unsafe.Pointer) {
	src := (*complex64)(p)
	dst := (*complex64)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_complex64
}

func encode_complex128(v *Value, p unsafe.Pointer) {
	src := (*complex128)(p)
	dst := (*complex128)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_complex128
}

func encode_array(v *Value, p unsafe.Pointer) {
	arr := (*reflect.Value)(p)
	op := enc_op_table[arr.Type().Elem().Kind()]

	length := arr.Len()
	for i := 0; i < length; i++ {
		src := unsafe.Pointer(arr.Index(i).UnsafeAddr())
		op(v, src)
	}
}

func encode_ptr(v *Value, p unsafe.Pointer) {
	src := (*uintptr)(p)
	dst := (*uintptr)(unsafe.Pointer(&v.b[v.idx]))
	*dst = *src
	v.idx += sz_uintptr
}

func encode_slice(v *Value, p unsafe.Pointer) {
	slice := (*reflect.SliceHeader)(p)
	encode_int(v, unsafe.Pointer(&slice.Len))
	encode_ptr(v, unsafe.Pointer(&slice.Data))
}

func encode_string(v *Value, p unsafe.Pointer) {
	s := *(*string)(p)
	cstr := C.CString(s)
	v.cstrings[v.idx] = cstr
	encode_ptr(v, unsafe.Pointer(cstr))
}

func encode_struct(v *Value, p unsafe.Pointer) {
	rv := (*reflect.Value)(p)
	nfields := rv.NumField()
	for i := 0; i < nfields; i++ {
		f := rv.Field(i)
		encode_value(v, f)
	}
}

func encode_value(cv *Value, rv reflect.Value) {

	kind := rv.Type().Kind()
	op := enc_op_table[kind]
	switch kind {
	default:
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
	case reflect.Array:
		op(cv, unsafe.Pointer(&rv))
	case reflect.Ptr:
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
	case reflect.Slice:
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
	case reflect.Struct:
		op(cv, unsafe.Pointer(&rv))
	case reflect.String:
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
	}
}

// A Decoder is bound to a particular reflect.Type and knows how to
// convert a ctypes.Value into a Go-value
type Decoder interface {
	Decode(v interface{}) (*Value, os.Error)
}

type ctype_decoder struct {
	v *Value // the C-value from which we decode
}

// Create a new decoder bound to the c-value v
func NewDecoder(v *Value) Decoder {
	v.idx = 0
	return &ctype_decoder{v: v}
}

// Decode a ctypes.Value into a Go value
func (d *ctype_decoder) Decode(v interface{}) (*Value, os.Error) {
	rv := follow_ptr(reflect.ValueOf(v))
	rt := rv.Type()
	if rt != d.v.Type().GoType() {
		return nil, os.NewError("cannot decode this type [" + rt.String() + "]")
	}
	d.v.Reset()
	decode_value(d.v, rv)
	return d.v, nil
}

type dec_op func(v *Value, p unsafe.Pointer)
var dec_op_table []dec_op

func decode_noop(v *Value, p unsafe.Pointer) {
	panic("noop!")
}

func decode_bool(v *Value, p unsafe.Pointer) {
	src := (*bool)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*bool)(p)
	*dst = *src
	v.idx += sz_bool
}

func decode_int(v *Value, p unsafe.Pointer) {
	src := (*int)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*int)(p)
	*dst = *src
	v.idx += sz_int
}

func decode_int8(v *Value, p unsafe.Pointer) {
	src := (*int8)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*int8)(p)
	*dst = *src
	v.idx += sz_int8
}

func decode_int16(v *Value, p unsafe.Pointer) {
	src := (*int16)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*int16)(p)
	*dst = *src
	v.idx += sz_int16
}

func decode_int32(v *Value, p unsafe.Pointer) {
	src := (*int32)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*int32)(p)
	*dst = *src
	v.idx += sz_int32
}

func decode_int64(v *Value, p unsafe.Pointer) {
	src := (*int64)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*int64)(p)
	*dst = *src
	v.idx += sz_int64
}

func decode_uint(v *Value, p unsafe.Pointer) {
	src := (*uint)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*uint)(p)
	*dst = *src
	v.idx += sz_uint
}

func decode_uint8(v *Value, p unsafe.Pointer) {
	src := (*uint8)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*uint8)(p)
	*dst = *src
	v.idx += sz_uint8
}

func decode_uint16(v *Value, p unsafe.Pointer) {
	src := (*uint16)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*uint16)(p)
	*dst = *src
	v.idx += sz_uint16
}

func decode_uint32(v *Value, p unsafe.Pointer) {
	src := (*uint32)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*uint32)(p)
	*dst = *src
	v.idx += sz_uint32
}

func decode_uint64(v *Value, p unsafe.Pointer) {
	src := (*uint64)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*uint64)(p)
	*dst = *src
	v.idx += sz_uint64
}

func decode_uintptr(v *Value, p unsafe.Pointer) {
	src := (*uintptr)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*uintptr)(p)
	*dst = *src
	v.idx += sz_uintptr
}

func decode_float32(v *Value, p unsafe.Pointer) {
	src := (*float32)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*float32)(p)
	*dst = *src
	v.idx += sz_float32
}

func decode_float64(v *Value, p unsafe.Pointer) {
	src := (*float64)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*float64)(p)
	*dst = *src
	v.idx += sz_float64
}

func decode_complex64(v *Value, p unsafe.Pointer) {
	src := (*complex64)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*complex64)(p)
	*dst = *src
	v.idx += sz_complex64
}

func decode_complex128(v *Value, p unsafe.Pointer) {
	src := (*complex128)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*complex128)(p)
	*dst = *src
	v.idx += sz_complex128
}

func decode_array(v *Value, p unsafe.Pointer) {
	arr := (*reflect.Value)(p)
	op := dec_op_table[arr.Type().Elem().Kind()]

	length := arr.Len()
	for i := 0; i < length; i++ {
		dst := unsafe.Pointer(arr.Index(i).UnsafeAddr())
		op(v, dst)
	}
}

func decode_ptr(v *Value, p unsafe.Pointer) {
	src := (*uintptr)(unsafe.Pointer(&v.b[v.idx]))
	dst := (*uintptr)(p)
	*dst = *src
	v.idx += sz_uintptr
}

func decode_slice(v *Value, p unsafe.Pointer) {
	slice := (*reflect.SliceHeader)(p)
	decode_int(v, unsafe.Pointer(&slice.Len))
	decode_ptr(v, unsafe.Pointer(&slice.Data))
}

func decode_string(v *Value, p unsafe.Pointer) {
	
	s := C.GoString(v.cstrings[v.idx])
	src := (*reflect.StringHeader)(unsafe.Pointer(&s))
	dst := (*reflect.StringHeader)(p)
	dst.Data = src.Data
	dst.Len  = src.Len
	v.idx += sz_uintptr
}

func decode_struct(v *Value, p unsafe.Pointer) {
	rv := (*reflect.Value)(p)
	nfields := rv.NumField()
	for i := 0; i < nfields; i++ {
		f := rv.Field(i)
		decode_value(v, f)
	}
}

func decode_value(cv *Value, rv reflect.Value) {
	//println("rv:",rv.Type())
	kind := rv.Type().Kind()
	op := dec_op_table[kind]
	switch kind {
	default:
		//println("-->",kind.String())
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
		//println("<--",kind.String())
	case reflect.Array:
		op(cv, unsafe.Pointer(&rv))
	case reflect.Ptr:
		//println("++>",kind.String())
		op(cv, unsafe.Pointer(rv.Pointer()))
		//println("<++",kind.String())
	case reflect.Slice:
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
	case reflect.Struct:
		op(cv, unsafe.Pointer(&rv))
	case reflect.String:
		//println("==>",kind.String())
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
		//println("<==",kind.String())
	}
}


func init() {
	enc_op_table = []enc_op{
		reflect.Bool:       encode_bool,
		reflect.Int:        encode_int,
		reflect.Int8:       encode_int8,
		reflect.Int16:      encode_int16,
		reflect.Int32:      encode_int32,
		reflect.Int64:      encode_int64,
		reflect.Uint:       encode_uint,
		reflect.Uint8:      encode_uint8,
		reflect.Uint16:     encode_uint16,
		reflect.Uint32:     encode_uint32,
		reflect.Uint64:     encode_uint64,
		reflect.Uintptr:    encode_uintptr,
		reflect.Float32:    encode_float32,
		reflect.Float64:    encode_float64,
		reflect.Complex64:  encode_complex64,
		reflect.Complex128: encode_complex128,
		reflect.Array:      encode_array,
		reflect.Chan:       encode_noop,
		reflect.Func:       encode_noop,
		reflect.Interface:  encode_noop,
		reflect.Map:        encode_noop,
		reflect.Ptr:        encode_ptr,
		reflect.Slice:      encode_slice,
		reflect.String:     encode_string,
		reflect.Struct:     encode_struct,
	}

	dec_op_table = []dec_op{
		reflect.Bool:       decode_bool,
		reflect.Int:        decode_int,
		reflect.Int8:       decode_int8,
		reflect.Int16:      decode_int16,
		reflect.Int32:      decode_int32,
		reflect.Int64:      decode_int64,
		reflect.Uint:       decode_uint,
		reflect.Uint8:      decode_uint8,
		reflect.Uint16:     decode_uint16,
		reflect.Uint32:     decode_uint32,
		reflect.Uint64:     decode_uint64,
		reflect.Uintptr:    decode_uintptr,
		reflect.Float32:    decode_float32,
		reflect.Float64:    decode_float64,
		reflect.Complex64:  decode_complex64,
		reflect.Complex128: decode_complex128,
		reflect.Array:      decode_array,
		reflect.Chan:       decode_noop,
		reflect.Func:       decode_noop,
		reflect.Interface:  decode_noop,
		reflect.Map:        decode_noop,
		reflect.Ptr:        decode_ptr,
		reflect.Slice:      decode_slice,
		reflect.String:     decode_string,
		reflect.Struct:     decode_struct,
	}
}

// EOF
