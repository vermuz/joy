package defs

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/fatih/structtag"
	"github.com/pkg/errors"

	"github.com/matthewmueller/golly/golang/def"
	"github.com/matthewmueller/golly/golang/index"
	"github.com/matthewmueller/golly/golang/util"

	"golang.org/x/tools/go/loader"
)

// Interfacer method
type Interfacer interface {
	def.Definition
	ImplementedBy(method string) []Methoder
	DependenciesOf(method string) ([]def.Definition, error)
	Node() *ast.TypeSpec
	Methods() []def.InterfaceMethod
	FindMethod(name string) def.InterfaceMethod
}

var _ Interfacer = (*interfaces)(nil)
var _ def.InterfaceMethod = (*method)(nil)

type interfaces struct {
	exported   bool
	path       string
	name       string
	id         string
	deps       []def.Definition
	index      *index.Index
	methods    []*method
	node       *ast.TypeSpec
	kind       *types.Interface
	processed  bool
	methodDeps map[string]def.Definition
	imports    map[string]string
}

type method struct {
	name            string
	tag             *structtag.Tag
	rewriteFunction def.Definition
}

func (m *method) OriginalName() string {
	return m.name
}

func (m *method) Name() string {
	if m.tag != nil {
		return m.tag.Name
	}
	return m.name
}

func (m *method) RewriteFunction() def.Definition {
	return m.rewriteFunction
}

// Interface fn
func Interface(index *index.Index, info *loader.PackageInfo, gn *ast.GenDecl, n *ast.TypeSpec) (def.Definition, error) {
	obj := info.ObjectOf(n.Name)
	packagePath := obj.Pkg().Path()
	idParts := []string{packagePath, n.Name.Name}
	id := strings.Join(idParts, " ")

	// get the methods
	var methods []*method
	iface := n.Type.(*ast.InterfaceType)
	for _, m := range iface.Methods.List {
		tag, err := util.JSTag(m.Doc)
		if err != nil {
			return nil, err
		}

		for _, id := range m.Names {
			methods = append(methods, &method{
				name: id.Name,
				tag:  tag,
			})
		}
	}

	kind, ok := info.TypeOf(n.Type).(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("NewInterface: expected n.Type to be an interface")
	}

	return &interfaces{
		exported: obj.Exported(),
		path:     packagePath,
		name:     n.Name.Name,
		id:       id,
		index:    index,
		node:     n,
		methods:  methods,
		kind:     kind,
		imports:  map[string]string{},
	}, nil
}

func (d *interfaces) process() (err error) {
	state, e := process(d.index, d, d.node)
	if e != nil {
		return e
	}

	// copy state into function
	d.processed = true
	d.imports = state.imports
	d.deps = state.deps

	return nil
}

// interfaces don't include dependencies on their own
func (d *interfaces) Dependencies() (deps []def.Definition, err error) {
	if !d.processed {
		if e := d.process(); e != nil {
			return deps, errors.Wrap(e, "error processing")
		}
	}
	return d.deps, nil
}

func (d *interfaces) ID() string {
	return d.id
}

func (d *interfaces) Name() string {
	return d.name
}

func (d *interfaces) OriginalName() string {
	return d.name
}

func (d *interfaces) Path() string {
	return d.path
}

func (d *interfaces) Exported() bool {
	return false
}
func (d *interfaces) Omitted() bool {
	return true
}

func (d *interfaces) Node() *ast.TypeSpec {
	return d.node
}

func (d *interfaces) Kind() string {
	return "INTERFACE"
}

func (d *interfaces) Type() types.Type {
	return d.kind
}

// TODO: optimize
func (d *interfaces) ImplementedBy(m string) (defs []Methoder) {
	for _, def := range d.index.All() {
		method, ok := def.(Methoder)
		if !ok {
			continue
		}

		if method.OriginalName() != m {
			continue
		}

		// TODO: pointer is required for the deep interfaces test and
		// it works for the other tests, but will it always work?
		if types.Implements(types.NewPointer(method.Recv().Type()), d.kind) {
			defs = append(defs, method)
		}
	}

	return defs
}

func (d *interfaces) DependenciesOf(m string) (defs []def.Definition, err error) {
	return defs, nil
}

func (d *interfaces) Imports() map[string]string {
	// combine def imports with file imports
	imports := map[string]string{}
	for alias, path := range d.imports {
		imports[alias] = path
	}
	for alias, path := range d.index.GetImports(d.path) {
		imports[alias] = path
	}
	return imports
}

func (d *interfaces) FromRuntime() bool {
	return false
}

func (d *interfaces) Methods() (methods []def.InterfaceMethod) {
	for _, method := range d.methods {
		methods = append(methods, method)
	}
	return methods
}

func (d *interfaces) FindMethod(name string) def.InterfaceMethod {
	for _, method := range d.methods {
		if name == method.OriginalName() {
			return method
		}
	}
	return nil
}
