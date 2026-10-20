package main

import (
	"fmt"

	"example.com/shelf/internal/loans"
	"example.com/shelf/internal/store"
)

func main() {
	s := store.Open()
	fmt.Println(loans.Due(s))
}
