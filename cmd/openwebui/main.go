package main

import (
	"log"

	openwebui "github.com/xxnuo/open-coreui/backend/open_webui"
)

func main() {
	if err := openwebui.Run(); err != nil {
		log.Fatal(err)
	}
}
