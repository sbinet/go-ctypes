package main

import (
	"fmt"
	"reflect"
	"unsafe"

	"github.com/sbinet/go-ctypes/pkg/ctypes"
)

type Event struct {
	i int
	f float64
	a []float64
	s string
	b [10]float64
	t *T1
}

func (e *Event) String() string {
	return fmt.Sprintf("Event{i:%d, f:%f, a:%v, s:'%s', b:%v, t:%v}",
		e.i, e.f, e.a, e.s, e.b,  e.t)
}

type T1 struct {
	i0 int
	s0 string
	i1 int
	f0 float64
	f1 float32
	f2 float32
}

func (t1 *T1) String() string {
	return fmt.Sprintf("T1{i0=%v s0=\"%s\" i1=%v f0=%v f1=%v f2=%v}",
		t1.i0, t1.s0, t1.i1, t1.f0, t1.f1, t1.f2)
}

func inspect(t reflect.Type) {
	nfields := t.NumField()
	fmt.Printf("=== inspecting [%v] ===\n", t)
	for i := 0; i < nfields; i++ {
		f := t.Field(i)
		fmt.Printf(":: [%s] '%v' off:%d sz:%d al:%d fal:%d\n", 
			f.Name, f.Type, 
			f.Offset, f.Type.Size(),
			f.Type.Align(), f.Type.FieldAlign())
	}
	fmt.Printf("=== inspecting [%v] === [done]\n", t)
	
}

func cinspect(t ctypes.Type) {
	nfields := t.NumField()
	fmt.Printf("=== inspecting [ctypes.%v] ===\n", t)
	for i := 0; i < nfields; i++ {
		f := t.Field(i)
		fmt.Printf(":: [%s] '%v' off:%d sz:%d\n", 
			f.Name, f.Type, f.Offset, f.Type.Size())
	}
	fmt.Printf("=== inspecting [ctypes.%v] === [done]\n", t)
}

func ee(b []byte, v int) {
	src := (*int)(unsafe.Pointer(&v))
	dst := (*int)(unsafe.Pointer(&b[0]))
	*dst = *src
	//fmt.Printf("b: %v [%d]\n", b, len(b))
	b = b[unsafe.Sizeof(int(0)):]
	//fmt.Printf("b: %v [%d]\n", b, len(b))
}

func main() {
	
	fmt.Printf("===\n")
	e := Event{i:257012, f:42222222222222222., a:[]float64{1., 2., 3.}}
	e.s = "32 - 42"
	e.b[0] = 423333333. 
	e.b[1] = 1.
	e.b[2] = 2. 
	e.b[5] = 666.;
	e.t = &T1{i0:32, i1:42, s0:"foo - bar", f0:256., f1:666., f2:42.}
	fmt.Printf("e: %v\n", e)
	
	ty_e := reflect.TypeOf(e)
	fmt.Printf("ty_e: %v %d\n", ty_e, ty_e.Size())
	ct := ctypes.TypeOf(e)
	fmt.Printf("ct_e: %v %d\n", ct, ct.Size())

	fmt.Printf(":: inspecting main.Event...\n")
	inspect(ty_e)
	cinspect(ct)

	{
		fmt.Printf(":: inspecting main.T1...\n")
		ty_t1 := reflect.TypeOf(*e.t)
		inspect(ty_t1)
		ct_t1 := ctypes.TypeOf(*e.t)
		cinspect(ct_t1)
	}
	fmt.Printf("---\n")

	if true {
		fmt.Printf("=== encoding [Event]...\n")
		c_value := ctypes.ValueOf(&e)
		fmt.Printf("buf: %v\n", c_value.Buffer())
		c_enc := ctypes.NewEncoder(c_value)
		c_value,err := c_enc.Encode(&e)
		fmt.Printf("v: %v\n", &e)
		fmt.Printf("buf: %v\n", c_value.Buffer())
		fmt.Printf("err: %v\n", err)
		
		{
			vv := Event{}
			c_dec := ctypes.NewDecoder(c_value)
			c_vv, err := c_dec.Decode(&vv)
			fmt.Printf("v: %v\n", &vv)
			//fmt.Printf("buf: %v\n", c_value.Buffer())
			fmt.Printf("buf: %v\n", c_vv.Buffer())
			fmt.Printf("err: %v\n", err)
		}
	}


	if true {
		fmt.Printf("=== encoding [T1]...\n")
		v := T1{i0:32, s0:"foo - bar", i1:42, f0:256., f1:666., f2:42.}
		c_value := ctypes.ValueOf(&v)
		c_enc := ctypes.NewEncoder(c_value)
		c_value,err := c_enc.Encode(&v)
		fmt.Printf("v: %v\n", &v)
		fmt.Printf("buf: %v\n", c_value.Buffer())
		fmt.Printf("err: %v\n", err)
		
		{
			vv := T1{}
			c_dec := ctypes.NewDecoder(c_value)
			c_vv, err := c_dec.Decode(&vv)
			fmt.Printf("v: %v\n", &vv)
			fmt.Printf("buf: %v\n", c_value.Buffer())
			fmt.Printf("buf: %v\n", c_vv.Buffer())
			fmt.Printf("err: %v\n", err)
		}
	}
	fmt.Printf("===\n")

	fmt.Printf(":: bye\n")
}