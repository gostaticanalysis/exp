package a

func main() {
	var a interface{}
	_ = a.(int)    // want "must not do fource type assertion"
	_, _ = a.(int) // OK

	switch a := a.(type) { // OK
	case int:
		println(a)
	}
}
