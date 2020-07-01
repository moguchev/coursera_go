package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

// fmt.Printf("type: %T data: %+v\n", val, val)

type tpl struct {
	In        string
	PostCheck string
	AuthCheck string
	Name      string
	Recv      string
}

var (
	handlerTpl = template.Must(template.New("handlerTpl").Parse(
		`func (s *{{.Recv}}) handler{{.Name}}(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	{{.PostCheck}}
	{{.AuthCheck}}
	in, e := get{{.In}}(r)
	if e != nil {
		RespondWith(w, http.StatusBadRequest, nil, e.Error())
		return
	}
	err := validate{{.In}}(&in)
	
	if err != nil {
		RespondWith(w, http.StatusBadRequest, nil, err.Error())
		return
	}

	res, err := s.{{.Name}}(ctx, in)
	if err != nil {
		if e, ok := err.(ApiError); ok {
			RespondWith(w, e.HTTPStatus, nil, e.Error())
		} else {
			RespondWith(w, http.StatusInternalServerError, nil, err.Error())
		}
		return
	}
	RespondWith(w, http.StatusOK, res, "")
	return
}
`))
)

type Api struct {
	URL    string `json:"url"`
	Auth   bool   `json:"auth"`
	Method string `json:"method"`
}

type methodInfo struct {
	name string
	api  Api
	in   string
}

type fields struct {
	name    string
	fType   string
	options options
}

type options struct {
	required     bool
	paramname    string
	enum         []string
	defaultvalue string
	min          int
	hasmin       bool
	max          int
	hasmax       bool
}

func setOptions(settings string, opt *options) {
	opt.required = false
	opt.hasmax = false
	opt.hasmin = false
	nodes := strings.Split(settings, ",")
	for _, node := range nodes {
		if strings.HasPrefix(node, "required") {
			opt.required = true
		}
		if strings.HasPrefix(node, "paramname") {
			kv := strings.Split(node, "=")
			opt.paramname = kv[1]
		}
		if strings.HasPrefix(node, "enum") {
			kv := strings.Split(node, "=")
			opt.enum = strings.Split(kv[1], "|")
		}
		if strings.HasPrefix(node, "default") {
			kv := strings.Split(node, "=")
			opt.defaultvalue = kv[1]
		}
		if strings.HasPrefix(node, "min") {
			kv := strings.Split(node, "=")
			opt.min, _ = strconv.Atoi(kv[1])
			opt.hasmin = true
		}
		if strings.HasPrefix(node, "max") {
			kv := strings.Split(node, "=")
			opt.max, _ = strconv.Atoi(kv[1])
			opt.hasmax = true
		}
	}
}

type genMethods map[string][]methodInfo

func baseTypeName(x ast.Expr) (name string) {
	switch t := x.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if _, ok := t.X.(*ast.Ident); ok {
			// only possible for qualified type names;
			// aasume type is imported
			return t.Sel.Name
		}
		break
	case *ast.StarExpr:
		return baseTypeName(t.X)
	}
	return
}

func main() {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := os.Create(os.Args[2])

	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out) // empty line
	fmt.Fprintln(out, `import "context"`)
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "fmt"`)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "strconv"`)

	fmt.Fprintln(out) // empty line

	apis := make(genMethods)
	models := make([]string, 0)
	for _, f := range node.Decls {
		fun, ok := f.(*ast.FuncDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.FuncDecl\n", f)
			continue
		}

		needCodegen := false
		funAPI := Api{}
		if fun.Doc != nil {
			for _, comment := range fun.Doc.List {
				hasMark := strings.HasPrefix(comment.Text, "// apigen:api")
				needCodegen = needCodegen || hasMark
				if hasMark {
					begin := strings.Index(comment.Text, "{")
					end := strings.LastIndex(comment.Text, "}")
					jsonStr := comment.Text[begin : end+1]
					bytes := []byte(jsonStr)
					json.Unmarshal(bytes, &funAPI)
				}
			}
		}

		if !needCodegen {
			fmt.Printf("SKIP function %#v does not have apigen:api mark\n", fun.Name.String())
			continue
		} else {
			if fun.Recv == nil {
				fmt.Printf("SKIP function %#v - is not method\n", fun.Name.String())
				continue
			} else {
				recv := baseTypeName(fun.Recv.List[0].Type)
				fmt.Printf("NEED GEN method %#v of %#v\n", fun.Name.String(), recv)

				if apis[recv] == nil {
					apis[recv] = make([]methodInfo, 0)
				}

				info := methodInfo{}
				info.name = fun.Name.String()
				info.api = funAPI
				info.in = baseTypeName(fun.Type.Params.List[1].Type)
				models = append(models, info.in)
				apis[recv] = append(apis[recv], info)
			}
		}
	}
	fmt.Fprintln(out,
		`func RespondWith(w http.ResponseWriter, status int, body interface{}, err string) {
	res := make(map[string]interface{})
	res["error"] = err
	if err == "" {
		res["response"] = body
	}
	bytes, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(bytes)
}`)
	fmt.Fprintln(out)

	// ServeHTTP
	for k, v := range apis {
		fmt.Printf("key[%s]\n", k)

		fmt.Fprintln(out, "func (s *"+k+") ServeHTTP(w http.ResponseWriter, r *http.Request) {")
		fmt.Fprintln(out, "\tswitch r.URL.Path {")
		for _, s := range v {
			fmt.Printf("%#v\n", s)

			fmt.Fprintf(out, "\t"+`case "%s":`+"\n", s.api.URL)
			fmt.Fprintln(out, "\t\ts.handler"+s.name+"(w, r)\n\t\tbreak")
		}
		fmt.Fprintln(out, "\tdefault:")
		fmt.Fprintln(out, "\t\t"+`RespondWith(w, http.StatusNotFound, nil, "unknown method")`)
		fmt.Fprintln(out, "\t\tbreak")
		fmt.Fprintln(out, "\t}")
		fmt.Fprintln(out, "}")
		fmt.Fprintln(out)
	}
	// handlers
	for k, v := range apis {
		for _, s := range v {
			var post string
			if s.api.Method == "POST" {
				post = `
	if r.Method != http.MethodPost {
		RespondWith(w, http.StatusNotAcceptable, nil, "bad method")
		return
	}
	`
			}
			auth := ""
			if s.api.Auth {
				auth = `
	auth := r.Header.Get("X-Auth")
	if auth != "100500" {
		RespondWith(w, http.StatusForbidden, nil, "unauthorized")
		return 
	}
`
			}
			handlerTpl.Execute(out, tpl{
				In:        s.in,
				PostCheck: post,
				AuthCheck: auth,
				Name:      s.name,
				Recv:      k,
			})
			fmt.Fprintln(out)
		}
	}
	// geters
	for _, f := range node.Decls {
		g, ok := f.(*ast.GenDecl)
		if !ok {
			fmt.Printf("SKIP %T is not *ast.GenDecl\n", f)
			continue
		}
	SPECS_LOOP:
		for _, spec := range g.Specs {
			currType, ok := spec.(*ast.TypeSpec)
			if !ok {
				fmt.Printf("SKIP %T is not ast.TypeSpec\n", spec)
				continue
			}

			currStruct, ok := currType.Type.(*ast.StructType)
			if !ok {
				fmt.Printf("SKIP %T is not ast.StructType\n", currStruct)
				continue
			}

			needCodegen := false
			for _, name := range models {
				needCodegen = needCodegen || (name == currType.Name.Name)
			}
			if !needCodegen {
				fmt.Printf("SKIP struct %#v doesnt have cgen mark\n", currType.Name.Name)
				continue SPECS_LOOP
			}

			fs := make([]fields, 0)
			// FIELDS_LOOP:
			for _, field := range currStruct.Fields.List {
				opt := options{}
				if field.Tag != nil {
					tag := reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
					settings := tag.Get("apivalidator")
					setOptions(settings, &opt)
					if opt.paramname == "" {
						opt.paramname = strings.ToLower(field.Names[0].Name)
					}
				}
				fs = append(fs, fields{
					name:    field.Names[0].Name,
					fType:   baseTypeName(field.Type),
					options: opt,
				})
			}

			fmt.Printf("process struct %s\n", currType.Name.Name)
			fmt.Printf("\tgenerating Get method\n")

			fmt.Fprintln(out, "func get"+currType.Name.Name+"(r *http.Request) ("+currType.Name.Name+", error) {")
			fmt.Fprintln(out, "\tin := "+currType.Name.Name+"{}")
			fmt.Fprintln(out, "\tvar err error")
			fmt.Fprintln(out, "\n\tif r.Method == http.MethodPost {")
			fmt.Fprintln(out, "\t\tr.ParseForm()")

			for _, f := range fs {
				if f.fType == "string" {
					fmt.Fprintf(out, "\t\t"+`in.%s = r.FormValue("%s")`+"\n", f.name, f.options.paramname)
				} else {
					fmt.Fprintf(out, "\t\t"+`in.%s, err = strconv.Atoi(r.FormValue("%s"))`+"\n", f.name, f.options.paramname)
					fmt.Fprintln(out, "\t\tif err != nil {")
					fmt.Fprintln(out, "\t\t\treturn in, fmt.Errorf(\""+f.options.paramname+" must be int\")")
					fmt.Fprintln(out, "\t\t}")
				}
			}
			fmt.Fprintln(out, "\t} else {")

			for _, f := range fs {
				if f.fType == "string" {
					fmt.Fprintf(out, "\t\t"+`in.%s = r.URL.Query().Get("%s")`+"\n", f.name, f.options.paramname)
				} else {
					fmt.Fprintf(out, "\t\t"+`in.%s, err = strconv.Atoi(r.URL.Query().Get("%s"))`+"\n", f.name, f.options.paramname)
					fmt.Fprintln(out, "\t\tif err != nil {")
					fmt.Fprintln(out, "\t\t\treturn in, fmt.Errorf(\""+f.options.paramname+" must be int\")")
					fmt.Fprintln(out, "\t\t}")
				}
			}
			fmt.Fprintln(out, "\t}")
			fmt.Fprintln(out, "\treturn in, err")
			fmt.Fprintln(out, "}")
			fmt.Fprintln(out)

			fmt.Printf("\tgenerating validation method\n")

			fmt.Fprintln(out, "func validate"+currType.Name.Name+"(in *"+currType.Name.Name+") error {")
			for _, f := range fs {
				if f.options.required {
					fmt.Fprintf(out, `
	if in.%s == "" {
		return fmt.Errorf("%s must me not empty")
	}
	`, f.name, f.options.paramname)
				}
				if f.options.defaultvalue != "" {
					if f.fType == "string" {
						fmt.Fprintf(out, `
	if in.%s == "" {
		in.%s = "%s"
	}`, f.name, f.name, f.options.defaultvalue)
					}
					if f.fType == "int" {
						fmt.Fprintf(out, `
						if in.%s == "" {
							in.%s = %s
						}`, f.name, f.name, f.options.defaultvalue)
					}
				}
				fmt.Fprintln(out)
				if len(f.options.enum) > 0 {
					enum := ""
					str := ""
					for _, e := range f.options.enum {
						enum += `"` + e + `", `
						str += e + ", "
					}
					enum = enum[0 : len(enum)-2]
					str = str[0 : len(str)-2]
					fmt.Fprintln(out, "\tok := false")
					fmt.Fprintln(out, "\tenum := []string{"+enum+"}")
					fmt.Fprintln(out, "\tfor _, e := range enum {")
					fmt.Fprintln(out, "\t\tif in."+f.name+" == e {")
					fmt.Fprintln(out, "\t\t\tok = true")
					fmt.Fprintln(out, "\t\t\tbreak")
					fmt.Fprintln(out, "\t\t}")
					fmt.Fprintln(out, "\t}")
					fmt.Fprintln(out, "\tif !ok {")

					fmt.Fprintln(out, "\t\treturn fmt.Errorf(\""+f.options.paramname+" must be one of [%s]\", `"+str+"`)")
					fmt.Fprintln(out, "\t}")
				}
				if f.options.hasmax {
					if f.fType == "int" {
						fmt.Fprintf(out, "\tif in.%s > %d {\n", f.name, f.options.max)
						fmt.Fprintf(out, "\t\t"+`return fmt.Errorf("%s must be <= %d")`, f.options.paramname, f.options.max)
						fmt.Fprintln(out)
						fmt.Fprintln(out, "\t}")
					}
					if f.fType == "string" {
						fmt.Fprintf(out, "\tif len(in.%s) > %d {\n", f.name, f.options.max)
						fmt.Fprintf(out, "\t\t"+`return fmt.Errorf("%s len must be <= %d")`, f.options.paramname, f.options.max)
						fmt.Fprintln(out)
						fmt.Fprintln(out, "\t}")
					}
				}
				if f.options.hasmin {
					if f.fType == "int" {
						fmt.Fprintf(out, "\tif in.%s < %d {\n", f.name, f.options.min)
						fmt.Fprintf(out, "\t\t"+`return fmt.Errorf("%s must be >= %d")`, f.options.paramname, f.options.min)
						fmt.Fprintln(out)
						fmt.Fprintln(out, "\t}")
					}
					if f.fType == "string" {
						fmt.Fprintf(out, "\tif len(in.%s) < %d {\n", f.name, f.options.min)
						fmt.Fprintf(out, "\t\t"+`return fmt.Errorf("%s len must be >= %d")`, f.options.paramname, f.options.min)
						fmt.Fprintln(out)
						fmt.Fprintln(out, "\t}")
					}
				}
			}
			fmt.Fprintln(out, "\treturn nil")
			fmt.Fprintln(out, "}")
			fmt.Fprintln(out)
		}
	}
}
