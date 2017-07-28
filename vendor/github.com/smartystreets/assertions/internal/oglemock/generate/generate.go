// Copyright 2012 Aaron Jacobs. All Rights Reserved.
// Author: aaronjjacobs@gmail.com (Aaron Jacobs)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package generate implements code generation for mock classes. This is an
// implementation detail of the createmock command, which you probably want to
// use directly instead.
package generate

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"path"
	"reflect"
	"regexp"
	"text/template"
)

const gTmplStr = `
// This file was auto-generated using createmock. See the following page for
// more information:
//
//     https://github.com/smartystreets/assertions/internal/oglemock
//

package {{pathBase .OutputPkgPath}}

import (
	{{range $identifier, $import := .Imports}}{{$identifier}} "{{$import}}"
	{{end}}
)

{{range .Interfaces}}
	{{$interfaceName := printf "Mock%s" .Name}}
	{{$structName := printf "mock%s" .Name}}

	type {{$interfaceName}} interface {
		{{getTypeString .}}
		oglemock.MockObject
	}

	type {{$structName}} struct {
		controller oglemock.Controller
		description string
	}
	
	func New{{printf "Mock%s" .Name}}(
		c oglemock.Controller,
		desc string) {{$interfaceName}} {
	  return &{{$structName}}{
			controller: c,
			description: desc,
		}
	}
	
	func (m *{{$structName}}) Oglemock_Id() uintptr {
		return uintptr(unsafe.Pointer(m))
	}
	
	func (m *{{$structName}}) Oglemock_Description() string {
		return m.description
	}

	{{range getMethods .}}
	  {{$funcType := .Type}}
	  {{$inputTypes := getInputs $funcType}}
	  {{$outputTypes := getOutputs $funcType}}

		func (m *{{$structName}}) {{.Name}}({{range $i, $type := $inputTypes}}p{{$i}} {{getInputTypeString $i $funcType}}, {{end}}) ({{range $i, $type := $outputTypes}}o{{$i}} {{getTypeString $type}}, {{end}}) {
			// Get a file name and line number for the caller.
			_, file, line, _ := runtime.Caller(1)

			// Hand the call off to the controller, which does most of the work.
			retVals := m.controller.HandleMethodCall(
				m,
				"{{.Name}}",
				file,
				line,
				[]interface{}{ {{range $i, $type := $inputTypes}}p{{$i}}, {{end}} })

			if len(retVals) != {{len $outputTypes}} {
				panic(fmt.Sprintf("{{$structName}}.{{.Name}}: invalid return values: %v", retVals))
			}

			{{range $i, $type := $outputTypes}}
				// o{{$i}} {{getTypeString $type}}
				if retVals[{{$i}}] != nil {
					o{{$i}} = retVals[{{$i}}].({{getTypeString $type}})
				}
			{{end}}

			return
		}
	{{end}}
{{end}}
`

type tmplArg struct {
	// The set of interfaces to mock, and the full name of the package from which
	// they all come.
	Interfaces       []reflect.Type
	InterfacePkgPath string

	// The package path for the generate code.
	OutputPkgPath string

	// Imports needed by the interfaces.
	Imports importMap
}

func (a *tmplArg) getInputTypeString(i int, ft reflect.Type) string {
	numInputs := ft.NumIn()
	if i == numInputs-1 && ft.IsVariadic() {
		return "..." + a.getTypeString(ft.In(i).Elem())
	}

	return a.getTypeString(ft.In(i))
}

func (a *tmplArg) getTypeString(t reflect.Type) string {
	return typeString(t, a.OutputPkgPath)
}

func getMethods(it reflect.Type) []reflect.Method {
	numMethods := it.NumMethod()
	methods := make([]reflect.Method, numMethods)

	for i := 0; i < numMethods; i++ {
		methods[i] = it.Method(i)
	}

	return methods
}

func getInputs(ft reflect.Type) []reflect.Type {
	numIn := ft.NumIn()
	inputs := make([]reflect.Type, numIn)

	for i := 0; i < numIn; i++ {
		inputs[i] = ft.In(i)
	}

	return inputs
}

func getOutputs(ft reflect.Type) []reflect.Type {
	numOut := ft.NumOut()
	outputs := make([]reflect.Type, numOut)

	for i := 0; i < numOut; i++ {
		outputs[i] = ft.Out(i)
	}

	return outputs
}

// A map from import identifier to package to use that identifier for,
// containing elements for each import needed by a set of mocked interfaces.
type importMap map[string]string

var typePackageIdentifierRegexp = regexp.MustCompile(`^([\pL_0-9]+)\.[\pL_0-9]+$`)

// Add an import for the supplied type, without recursing.
func addImportForType(imports importMap, t reflect.Type) {
	// If there is no package path, this is a built-in type and we don't need an
	// import.
	pkgPath := t.PkgPath()
	if pkgPath == "" {
		return
	}

	// Work around a bug in Go:
	//
	//     http://code.google.com/p/go/issues/detail?id=2660
	//
	var errorPtr *error
	if t == reflect.TypeOf(errorPtr).Elem() {
		return
	}

	// Use the identifier that's part of the type's string representation as the
	// import identifier. This means that we'll do the right thing for package
	// "foo/bar" with declaration "package baz".
	match := typePackageIdentifierRegexp.FindStringSubmatch(t.String())
	if match == nil {
		return
	}

	imports[match[1]] = pkgPath
}

// Add all necessary imports for the type, recursing as appropriate.
func addImportsForType(imports importMap, t reflect.Type) {
	// Add any import needed for the type itself.
	addImportForType(imports, t)

	// Handle special cases where recursion is needed.
	switch t.Kind() {
	case reflect.Array, reflect.Chan, reflect.Ptr, reflect.Slice:
		addImportsForType(imports, t.Elem())

	case reflect.Func:
		// Input parameters.
		for i := 0; i < t.NumIn(); i++ {
			addImportsForType(imports, t.In(i))
		}

		// Return values.
		for i := 0; i < t.NumOut(); i++ {
			addImportsForType(imports, t.Out(i))
		}

	case reflect.Map:
		addImportsForType(imports, t.Key())
		addImportsForType(imports, t.Elem())
	}
}

// Add imports for each of the methods of the interface, but not the interface
// itself.
func addImportsForInterfaceMethods(imports importMap, it reflect.Type) {
	// Handle each method.
	for i := 0; i < it.NumMethod(); i++ {
		m := it.Method(i)
		addImportsForType(imports, m.Type)
	}
}

// Given a set of interfaces, return a map from import identifier to package to
// use that identifier for, containing elements for each import needed by the
// mock versions of those interfaces in a package with the given path.
func getImports(
	interfaces []reflect.Type,
	pkgPath string) importMap {
	imports := make(importMap)
	for _, it := range interfaces {
		addImportForType(imports, it)
		addImportsForInterfaceMethods(imports, it)
	}

	// Make sure there are imports for other types used by the generated code
	// itself.
	imports["fmt"] = "fmt"
	imports["oglemock"] = "github.com/smartystreets/assertions/internal/oglemock"
	imports["runtime"] = "runtime"
	imports["unsafe"] = "unsafe"

	// Remove any self-imports generated above.
	for k, v := range imports {
		if v == pkgPath {
			delete(imports, k)
		}
	}

	return imports
}

// Given a set of interfaces to mock, write out source code suitable for
// inclusion in a package with the supplied full package path containing mock
// implementations of those interfaces.
func GenerateMockSource(
	w io.Writer,
	outputPkgPath string,
	interfaces []reflect.Type) (err error) {
	// Sanity-check arguments.
	if outputPkgPath == "" {
		return errors.New("Package path must be non-empty.")
	}

	if len(interfaces) == 0 {
		return errors.New("List of interfaces must be non-empty.")
	}

	// Make sure each type is indeed an interface.
	for _, it := range interfaces {
		if it.Kind() != reflect.Interface {
			return errors.New("Invalid type: " + it.String())
		}
	}

	// Make sure each interface is from the same package.
	interfacePkgPath := interfaces[0].PkgPath()
	for _, t := range interfaces {
		if t.PkgPath() != interfacePkgPath {
			err = fmt.Errorf(
				"Package path mismatch: %q vs. %q",
				interfacePkgPath,
				t.PkgPath())

			return
		}
	}

	// Set up an appropriate template arg.
	arg := tmplArg{
		Interfaces:       interfaces,
		InterfacePkgPath: interfacePkgPath,
		OutputPkgPath:    outputPkgPath,
		Imports:          getImports(interfaces, outputPkgPath),
	}

	// Configure and parse the template.
	tmpl := template.New("code")
	tmpl.Funcs(template.FuncMap{
		"pathBase":           path.Base,
		"getMethods":         getMethods,
		"getInputs":          getInputs,
		"getOutputs":         getOutputs,
		"getInputTypeString": arg.getInputTypeString,
		"getTypeString":      arg.getTypeString,
	})

	_, err = tmpl.Parse(gTmplStr)
	if err != nil {
		err = fmt.Errorf("Parse: %v", err)
		return
	}

	// Execute the template, collecting the raw output into a buffer.
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, arg); err != nil {
		return err
	}

	// Parse the output.
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(
		fset,
		path.Base(outputPkgPath+".go"),
		buf,
		parser.ParseComments)

	if err != nil {
		err = fmt.Errorf("parser.ParseFile: %v", err)
		return
	}

	// Sort the import lines in the AST in the same way that gofmt does.
	ast.SortImports(fset, astFile)

	// Pretty-print the AST, using the same options that gofmt does by default.
	cfg := &printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8,
	}

	if err = cfg.Fprint(w, fset, astFile); err != nil {
		return errors.New("Error pretty printing: " + err.Error())
	}

	return nil
}
