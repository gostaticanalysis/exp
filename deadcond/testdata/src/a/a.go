package a

var N int

func f() {
	if N == 0 {
		println("N == 0")
	}

	if N != 0 { // want "nooo"
		println("N != 0")
	}
}
