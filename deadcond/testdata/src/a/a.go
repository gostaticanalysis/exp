package a

var N int

func g() int {
	return N
}

func f() {

	const zero = 0
	n := g()
	if n <= zero {
		println("n == 0")
		return
	}

	if n >= zero { // -want "n >= zero is always true"
		println("n != 0")
	}

	m := 10
	var i int
	for { // ignore for loop
		i++
		m -= i
		if m < 0 {
			break
		}
	}
}
