package a

var N int

func g() int {
	return N
}

func f() {

	const zero = 0
	n := g()
	if n == zero {
		println("n == 0")
	}

	if n != zero { // want "n != zero must be true"
		println("n != 0")
	}
}
