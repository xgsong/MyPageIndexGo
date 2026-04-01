package main

import (
	"log"

	"github.com/xgsong/mypageindexgo/pkg/mcp"
)

func main() {
	if err := mcp.Run(); err != nil {
		log.Fatal(err)
	}
}
