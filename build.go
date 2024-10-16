package main

import (
	"fmt"
	"html/template"
	"os"
)

func parse(path string) {
	t, err := template.ParseFiles(path)
	if err != nil {
		fmt.Println(err)
		return
	}

	f, err := os.Create(path)
	if err != nil {
		fmt.Println("create file: ", err)
		return
	}

	// A sample config
	config := map[string]string{
		"textColor":      "#abcdef",
		"linkColorHover": "#ffaacc",
	}

	err = t.Execute(f, config)
	if err != nil {
		fmt.Println("execute: ", err)
		return
	}
	f.Close()
}

func main() {
	os.Mkdir("public", 0755)
	// parse("t.html")
}
