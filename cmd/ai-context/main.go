package main

import (
	"os"

	"github.com/zhuzepeng/ai-context/internal/aictx"
)

func main() {
	os.Exit(aictx.Run(os.Args[1:], os.Stdout, os.Stderr))
}
