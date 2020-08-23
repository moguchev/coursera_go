package main

import (
	"fmt"
	"io"
	"os"
	p "path"
	"sort"
	"strconv"
)

const (
	endSign  = "└───"
	dirSign  = "├───"
	pipeSign = "│\t"
)

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	return dirListRecursive(out, path, printFiles, "", "", true)
}

func dirListRecursive(out io.Writer, path string, printFiles bool,
	prefix string, last string, isFirstLevel bool) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()

	fileInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		if !isFirstLevel {
			fmt.Fprintln(out, prefix+last+fileInfo.Name())
		}

		subFiles, err := getSortedSubFilesToPrint(dir, printFiles)
		if err != nil {
			return err
		}

		for i, subFName := range subFiles {
			var sPref, sLast string

			if !isFirstLevel {
				if last == endSign {
					sPref = "\t"
				} else {
					sPref = pipeSign
				}
			}

			if i == len(subFiles)-1 {
				sLast = endSign
			} else {
				sLast = dirSign
			}
			// path + string(os.PathSeparator) + subFName
			dirListRecursive(out, p.Join(path, subFName), printFiles, prefix+sPref, sLast, false)
		}
	} else if printFiles && !isFirstLevel {
		printFile(out, fileInfo, prefix, last)
	}

	return nil
}

func printFile(out io.Writer, f os.FileInfo, prefix, last string) {
	var fSize string
	if f.Size() == 0 {
		fSize = "empty"
	} else {
		fSize = strconv.FormatInt(f.Size(), 10) + "b"
	}
	fmt.Fprintf(out, "%s (%s)\n", prefix+last+f.Name(), fSize)
}

func getSortedSubFilesToPrint(f *os.File, print bool) ([]string, error) {
	subFilesTmp, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}

	var subFiles []string
	for _, subFile := range subFilesTmp {
		if !(print || subFile.IsDir()) {
			continue
		}
		subFiles = append(subFiles, subFile.Name())
	}

	sort.Strings(subFiles)
	return subFiles, nil
}
