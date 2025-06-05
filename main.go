package noture

import (
	"log"
	"net/http"
)

func main() {
	PORT := "127.0.0.1:8080"
	log.Fatal(http.ListenAndServe(PORT, nil))
}
