package main

// это программа для которой ваш кодогенератор будет писать код
// запускать через go test -v, как обычно

import (
	"fmt"
	"net/http"
)

func main() {
	// будет вызван метод ServeHTTP у структуры MyApi
	http.Handle("/user/", NewMyApi())

	fmt.Println("starting server at :8080")
	http.ListenAndServe(":8080", nil)
}
