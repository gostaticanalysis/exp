package a

var N int

func g() int {
	return N
}

func f(s string) {
	//const zero = 0
	//n := g()
	//if n == zero {
	//	println("n == 0")
	//	return
	//}

	//if n != zero { // -want "n != zero is always true"
	//	println("n != 0")
	//}

	//m := g()
	//var i int
	//for m+i < 10 { // ignore for loop
	//	i++
	//}

	//for i := 0; i < 2; i++ {
	for range []int{1, 2} {
		n := g()
		if n == 0 {
			print("hoge")
		}
	}
}
