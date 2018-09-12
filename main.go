package main

import (
	"log"

	"github.com/ninjadotorg/SimEcon/economy"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// file := flag.String("f", "", "economy description file")
	// flag.Parse()

	// if e := economy.Run(*file); e != nil {
	// 	log.Println(e)
	// }

	economy.Run("abc")
}
