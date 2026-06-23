package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
)

func main() {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, os.Args[1], nil, parser.AllErrors)
	if err != nil {
		fmt.Println(err)
	}
}
