package ctypes

/*
*/
import "C"

import (
	"reflect"
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
	Bool = Kind(reflect.Bool)
	Int = Kind(reflect.Int)
	Int8 = Kind(reflect.Int8)
	Int16 = Kind(reflect.Int16)
	Int32 = Kind(reflect.Int32)
	Int64 = Kind(reflect.Int64)
	Uint = Kind(reflect.Uint)
	Uint8 = Kind(reflect.Uint8)
	Uint16 = Kind(reflect.Uint16)
	Uint32 = Kind(reflect.Uint32)
	Uint64 = Kind(reflect.Uint64)
	Uintptr = Kind(reflect.Uintptr)
	Float32 = Kind(reflect.Float32)
	Float64 = Kind(reflect.Float64)
	Complex64
	Complex128
	Array = Kind(reflect.Array)
	VLArray
	//Chan
	//Func
	//Interface
	//Map // <-- FIXME? can we implement this ?
	Ptr = Kind(reflect.Ptr)
	Slice  = VLArray
	String = Kind(reflect.String)
	Struct = Kind(reflect.Struct)
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

type Value struct {
	c []byte // the C value for that Value
	t Type   // the C type of that Value
}

func New(t Type) Value {
	if t == nil {
		panic("ctypes: New(nil)")
	}
	return Value{c:make([]byte, t.Size()), t:t}
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
func TypeFrom(t reflect.Type) Type {
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
	return TypeFrom(t.Type.Elem())
}

func (t *common_type) Field(i int) (c StructField) {
	f := t.Type.Field(i)
	c = StructField{
	PkgPath: f.PkgPath,
	Name: f.Name,
	Type: TypeFrom(f.Type),
	Tag: f.Tag,
	// FIXME?: this should be corrected for vlarrays/cstrings
	Offset: f.Offset,
	// FIXME?: ditto
	Index: f.Index,   
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
	fields map[string]Type
}

func new_cstruct(t reflect.Type) *cstruct_type {
	c := &cstruct_type{
	common_type:common_type{t}, 
	fields: make(map[string]Type)}

	nfields := t.NumField()
	for i := 0; i < nfields; i++ {
		f := t.Field(i)
		c.fields[f.Name] = TypeFrom(f.Type)
	}
	return c
}

func (t *cstruct_type) Size() uintptr {
	sz := uintptr(0)
	for _,v := range t.fields {
		// FIXME: alignment ?
		sz += v.Size()
	}
	return sz
}

// EOF
