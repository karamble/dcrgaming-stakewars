package main

import (
	"github.com/karamble/dcrgaming-stakewars/internal/analyzer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(analyzer.New(analyzer.SimPrefix)) }
