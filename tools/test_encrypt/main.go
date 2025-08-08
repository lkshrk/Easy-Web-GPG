package main

import (
	"fmt"
	cm "h-cloud.io/web-gpg/internal/crypto"
	"os"
)

func main() {
	fmt.Println("MASTER_KEY env set:", os.Getenv("MASTER_KEY") != "")
	s, err := cm.Encrypt([]byte("pass"))
	if err != nil {
		fmt.Println("encrypt err:", err)
		return
	}
	fmt.Println("encrypted ok, len:", len(s))
}
