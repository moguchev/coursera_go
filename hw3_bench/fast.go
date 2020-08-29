package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/moguchev/coursera_go/hw3_bench/models"
)

var android = []byte("Android")
var msie = []byte("MSIE")

// вам надо написать более быструю оптимальную этой функции
func FastSearch(out io.Writer) {
	// SlowSearch(out)
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	seenBrowsers := make(map[string]struct{})

	fmt.Fprintln(out, "found users:")

	user := models.User{}
	scanner := bufio.NewScanner(file)
	for i := 0; scanner.Scan(); i++ {
		if bytes.Contains(scanner.Bytes(), android) == false &&
			bytes.Contains(scanner.Bytes(), msie) == false {
			continue
		}

		err := user.UnmarshalJSON(scanner.Bytes())
		if err != nil {
			panic(err)
		}

		isAndroid := false
		isMSIE := false

		for _, browser := range user.Browsers {
			if strings.Contains(browser, "Android") {
				isAndroid = true
				seenBrowsers[browser] = struct{}{}
			}
			if strings.Contains(browser, "MSIE") {
				isMSIE = true
				seenBrowsers[browser] = struct{}{}
			}
		}

		if !(isAndroid && isMSIE) {
			continue
		}
		email := strings.Replace(user.Email, "@", " [at] ", 1)
		fmt.Fprintln(out, fmt.Sprintf("[%d] %s <%s>", i, user.Name, email))
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	fmt.Fprintln(out, "\nTotal unique browsers", len(seenBrowsers))
}

func main() {
	FastSearch(ioutil.Discard)

	fastOut := new(bytes.Buffer)
	FastSearch(fastOut)
	fmt.Print(fastOut.String())
}
