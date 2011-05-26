package ctypes

/*
 #include <string.h>
 #include <stdlib.h>
*/
import "C"

import (
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

const (
	Invalid Kind = iota
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
	Complex64
	Complex128
	Array = Kind(reflect.Array)
	VLArray
	//Chan
	//Func
	//Interface
	//Map // <-- FIXME? can we implement this ?
	Ptr           = Kind(reflect.Ptr)
	Slice         = VLArray
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

// get the C type corresponding to a Go type
func TypeOf(t reflect.Type) Type {
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
	return TypeOf(t.Type.Elem())
}

func (t *common_type) Field(i int) (c StructField) {
	f := t.Type.Field(i)
	c = StructField{
		PkgPath: f.PkgPath,
		Name:    f.Name,
		Type:    TypeOf(f.Type),
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
	return reflect.TypeOf(uintptr(0)).Size() + reflect.TypeOf(int(0)).Size()
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
		c.fields[f.Name] = TypeOf(f.Type)
	}
	return c
}

func (t *cstruct_type) Size() uintptr {
	sz := uintptr(0)
	for _, v := range t.fields {
		// FIXME: alignment ?
		sz += v.Size()
	}
	return sz
}


// An Encoder is bound to a particular reflect.Type and knows how to
// convert a Go value into a ctypes.Value
type Encoder interface {
	Encode(v interface{}) (*Value, os.Error)
}

func Encode(v interface{}) *Value {
	rv := reflect.ValueOf(v).Elem()
	rt := reflect.TypeOf(v).Elem()
	t := TypeOf(rt)
	c_value := New(t)

	encode_value(c_value, rv)
	return c_value
}

type ctype_encoder struct {
	v *Value // the C-value in which we encode
}

// Create a new encoder bound to the Go-type t
func NewEncoder(t interface{}) Encoder {
	rt := reflect.TypeOf(t).Elem()
	ct := TypeOf(rt)
	cv := New(ct)
	return &ctype_encoder{v: cv}
}

// Encode a Go value into a ctypes.Value
func (e *ctype_encoder) Encode(v interface{}) (*Value, os.Error) {
	rt := reflect.TypeOf(v).Elem()
	if rt != e.v.Type().GoType() {
		return nil, os.NewError("cannot encode this type [" + rt.String() + "]")
	}

	rv := reflect.ValueOf(v).Elem()
	e.v.Reset()
	encode_value(e.v, rv)
	return e.v, nil
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
	case reflect.Slice, reflect.Struct:
		op(cv, unsafe.Pointer(&rv))
	case reflect.String:
		op(cv, unsafe.Pointer(rv.UnsafeAddr()))
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
}

// EOF
