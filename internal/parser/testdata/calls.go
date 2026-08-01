package helpers

func helperA() {
	helperB()
	processItem()
}

func helperB() {
	processItem()
}

func processItem() {}
