package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/axw/gocov"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Coverage struct {
	XMLName    xml.Name  `xml:"coverage"`
	LineRate   float32   `xml:"line-rate,attr"`
	BranchRate float32   `xml:"branch-rate,attr"`
	Version    string    `xml:"version,attr"`
	Timestamp  int64     `xml:"timestamp,attr"`
	Packages   []Package `xml:"packages>package"`
	Sources    []string  `xml:"sources>source"`
}

type Package struct {
	Name       string  `xml:"name,attr"`
	LineRate   float32 `xml:"line-rate,attr"`
	BranchRate float32 `xml:"branch-rate,attr"`
	Complexity float32 `xml:"complexity,attr"`
	Classes    []Class `xml:"classes>class"`
}

type Class struct {
	Name       string   `xml:"name,attr"`
	Filename   string   `xml:"filename,attr"`
	LineRate   float32  `xml:"line-rate,attr"`
	BranchRate float32  `xml:"branch-rate,attr"`
	Complexity float32  `xml:"complexity,attr"`
	Methods    []Method `xml:"methods>method"`
	Lines      []Line   `xml:"lines>line"`
}

type Method struct {
	Name       string  `xml:"name,attr"`
	Signature  string  `xml:"signature,attr"`
	LineRate   float32 `xml:"line-rate,attr"`
	BranchRate float32 `xml:"branch-rate,attr"`
	Lines      []Line  `xml:"lines>line"`
}

type Line struct {
	Number int   `xml:"number,attr"`
	Hits   int64 `xml:"hits,attr"`
}

func main() {
	// check the arguments.
	bpath := "/"
	if len(os.Args) > 2 {
		if os.Args[1] == "-b" {
			bpath = os.Args[2]
		} else {
			panic("invalid arguments")
		}
	} else if len(os.Args) > 1 {
		if os.Args[1] == "-pwd" {
			bpath, _ = os.Getwd()
		} else {
			panic("invalid arguments")
		}
	}
	if !filepath.IsAbs(bpath) {
		panic(fmt.Sprintf("base path is abstruct path:%s", bpath))
	}
	sources := make([]string, 1)
	sources[0] = bpath
	//
	var r struct{ Packages []gocov.Package }
	err := json.NewDecoder(os.Stdin).Decode(&r)
	if err != nil {
		panic(err)
	}

	fset := token.NewFileSet()
	tokenFiles := make(map[string]*token.File)

	// convert packages
	packages := make([]Package, len(r.Packages))
	for i, gPackage := range r.Packages {
		// group functions by filename and "class" (type)
		files := make(map[string]map[string]*Class)
		for _, gFunction := range gPackage.Functions {
			// get the releative path by base path.
			fpath, _ := filepath.Rel(bpath, gFunction.File)
			classes := files[fpath]
			if classes == nil {
				// group functions by "class" (type) in a File
				classes = make(map[string]*Class)
				files[fpath] = classes
			}

			s := strings.Split("-."+gFunction.Name, ".") // className is "-" for package-level functions
			className, methodName := s[len(s)-2], s[len(s)-1]
			class := classes[className]
			if class == nil {
				class = &Class{Name: className, Filename: fpath, Methods: []Method{}, Lines: []Line{}}
				classes[className] = class
			}

			// from github.com/axw/gocov /gocov/annotate.go#printFunctionSource
			// Load the file for line information. Probably overkill, maybe
			// just compute the lines from offsets in here.
			setContent := false
			tokenFile := tokenFiles[gFunction.File]
			if tokenFile == nil {
				info, err := os.Stat(gFunction.File)
				if err != nil {
					panic(err)
				}
				tokenFile = fset.AddFile(gFunction.File, fset.Base(), int(info.Size()))
				setContent = true
			}

			tokenData, err := ioutil.ReadFile(gFunction.File)
			if err != nil {
				panic(err)
			}
			if setContent {
				// This processes the content and records line number info.
				tokenFile.SetLinesForContent(tokenData)
			}

			// convert statements to lines
			lines := make([]Line, len(gFunction.Statements))
			for i, s := range gFunction.Statements {
				lineno := tokenFile.Line(tokenFile.Pos(s.Start))
				line := Line{Number: lineno, Hits: s.Reached}
				lines[i] = line
				class.Lines = append(class.Lines, line)
			}

			class.Methods = append(class.Methods, Method{Name: methodName, Lines: lines})
		}

		// fill package with "classes"
		p := Package{Name: gPackage.Name, Classes: []Class{}}
		for _, classes := range files {
			for _, class := range classes {
				p.Classes = append(p.Classes, *class)
			}
		}
		packages[i] = p
	}

	coverage := Coverage{Sources: sources, Packages: packages, Timestamp: time.Now().UnixNano() / int64(time.Millisecond)}

	fmt.Printf(xml.Header)
	fmt.Printf("<!DOCTYPE coverage SYSTEM \"http://cobertura.sourceforge.net/xml/coverage-03.dtd\">\n")

	encoder := xml.NewEncoder(os.Stdout)
	encoder.Indent("", "\t")
	err = encoder.Encode(coverage)
	if err != nil {
		panic(err)
	}

	fmt.Println()
}
