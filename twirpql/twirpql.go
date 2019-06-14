package twirpql

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/99designs/gqlgen/api"
	gqlconfig "github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/plugin/modelgen"
	"github.com/golang/protobuf/proto"
	pgs "github.com/lyft/protoc-gen-star"
	pgsgo "github.com/lyft/protoc-gen-star/lang/go"
	"gopkg.in/yaml.v2"
	"marwan.io/protoc-gen-twirpql/internal/genenums"
	"marwan.io/protoc-gen-twirpql/internal/genresolver"
	"marwan.io/protoc-gen-twirpql/internal/genscalar"
	"marwan.io/protoc-gen-twirpql/internal/genserver"
	"marwan.io/protoc-gen-twirpql/internal/genunions"
	"marwan.io/protoc-gen-twirpql/internal/gqlfmt"
)

// twirpql creates a report of all the target messages generated by the
// protoc run, writing the file into the /tmp directory.
type twirpql struct {
	*pgs.ModuleBase

	// an input is a protobuf "message" that is
	// found inside an RPC's Request so that GraphQL
	// interprets it as an Input declaration.
	// Note that if the same input is also found
	// in an rpc's "returns" value, then the name
	// will be suffixed with the word "Input"
	// because GraphQL does not allow types and
	// inputs with matching names.
	inputs map[string]*serviceType

	// a "type" is a protobuf "message" that is
	// found inside an RPC's Return so that GraphQL
	// interprets it as a "Type" declaration.
	types map[string]*serviceType

	// a "union" represents a schema.graphql
	// Union definition which originates from
	// a protobuf `oneof` declaration inside
	// a message.
	unions     map[string]*union
	unionNames map[string]bool

	// an empty type keeps track of empty returns
	// because GraphQL Types can't be empty
	// and therefore we need to inject a dummy
	// field.
	emptys map[string]bool

	// enums are integers in protobuf but strings
	// in GraphQL. Therefore, we need to keep track
	// of declared enums in the proto file so that
	// we create proper conversion for the GraphQL queries.
	enums map[string]*enumData

	// maps are all map<type, type> declarations
	// in a protobuf file. Those get turned into
	// scalar values in GraphQL. The go type
	// here is a map of the scalar name (the field name)
	// to the full Go type representation.
	// For example if we have a protobuf that looks like
	// map<string, int64> myMap = 1;
	// Then this map would look like {"MyMap": "map[string]int64"}
	maps map[string]string

	// mapImports correspond to any import paths
	// the above maps field requires, such as
	// when the map ends up being something
	// like map[string]*ptypes.Timestamp
	mapImports map[string]struct{}

	// gqlTypes are specific for the gqlgen config file
	// so that we make all the input/output GraphQL
	// types point to the generated .pb.go types.
	gqlTypes gqlconfig.TypeMap

	// this is the graphql schema template
	tmpl *template.Template

	// this context holds Go related information about
	// a protobuf file.
	ctx pgsgo.Context

	// modname is the import path that "go list"
	// returns from inside the target .proto file
	modname string

	// gopkgname is the `option go_package` value
	gopkgname string

	// svcname is the name of the "service"
	// declaration  in a protofile.
	svcname string

	// destpkgname is the directory path
	// where the GraphQL generated code will
	// live. It defaults to a "twirpql".
	destpkgname string

	// is the import path that will import
	// the twirpql sub-package
	destimportpath string

	svc      pgs.Service
	protopkg pgs.Package
}

type enumData struct {
	Name        string
	ImportPath  string
	PackageName string
	Values      []string
}

// New configures the module with an instance of ModuleBase
func New(importPath string) pgs.Module {
	return &twirpql{
		ModuleBase:     &pgs.ModuleBase{},
		inputs:         map[string]*serviceType{},
		types:          map[string]*serviceType{},
		emptys:         map[string]bool{},
		enums:          map[string]*enumData{},
		maps:           map[string]string{},
		mapImports:     map[string]struct{}{},
		unions:         map[string]*union{},
		unionNames:     map[string]bool{},
		gqlTypes:       gqlconfig.TypeMap{},
		tmpl:           template.Must(template.New("").Funcs(schemaFuncs).Parse(schemaTemplate)),
		modname:        importPath,
		ctx:            pgsgo.InitContext(pgs.ParseParameters("")),
		destpkgname:    "./twirpql",
		destimportpath: "",
	}
}

// Name is the identifier used to identify the module. This value is
// automatically attached to the BuildContext associated with the ModuleBase.
func (tql *twirpql) Name() string { return "twirpql" }

func (tql *twirpql) InitContext(c pgs.BuildContext) {
	tql.ModuleBase.InitContext(c)
	tql.ctx = pgsgo.InitContext(c.Parameters())
}

// Execute is passed the target files as well as its dependencies in the pkgs
// map. The implementation should return a slice of Artifacts that represent
// the files to be generated. In this case, "/tmp/report.txt" will be created
// outside of the normal protoc flow.
func (tql *twirpql) Execute(targets map[string]pgs.File, pkgs map[string]pgs.Package) []pgs.Artifact {
	tql.destpkgname = tql.Parameters().StrDefault("dest", tql.destpkgname)
	os.MkdirAll(tql.destpkgname, 0777)

	if len(targets) != 1 {
		panic("only one proto file is supported at this moment; see https://twirpql.dev/docs/multiple-services")
	}

	for fileName, targetFile := range targets {
		if targetFile.Syntax() != pgs.Proto3 {
			panic("only proto3 is supported")
		}
		tql.svc = tql.pickServiceFromFile(tql.Parameters().Str("service"), targetFile)
		if len(tql.svc.Methods()) == 0 {
			panic("service must have at least on rpc")
		}
		tql.protopkg = targetFile.Package()
		serviceDir := filepath.Dir(fileName)
		tql.setImportPath(serviceDir)
		if serviceDir == "." {
			tql.destimportpath = tql.modname
		} else {
			tql.destimportpath = tql.goList(".")
		}
		f, err := os.Create(tql.path("schema.graphql"))
		must(err)
		defer f.Close()
		tql.generateSchema(targetFile, f)
	}

	if len(tql.maps) > 0 {
		f, err := os.Create(tql.path("scalars.go"))
		must(err)
		defer f.Close()
		must(genscalar.Render(tql.maps, tql.mapImports, f))
	}

	f, err := os.Create(tql.path("gqlgen.yml"))
	must(err)
	defer f.Close()
	tql.touchConfig(f)
	if len(tql.enums) > 0 {
		tql.bridgeEnums()
	}
	if len(tql.unions) > 0 {
		tql.writeUnionMask()
	}
	tql.initGql(tql.svcname)

	return tql.Artifacts()
}

func (tql *twirpql) pickServiceFromFile(svc string, f pgs.File) pgs.Service {
	switch len(f.Services()) {
	case 0:
		panic("proto file must have at least one service")
	case 1:
		return f.Services()[0]
	}
	if svc == "" {
		panic("service name must be provided if proto file has multiple services; see https://twirpql.dev/docs/multiple-services")
	}
	for _, service := range f.Services() {
		if svc == service.Name().String() {
			return service
		}
	}
	panic("protofile does not have the given service: " + svc)
}

func (tql *twirpql) goList(dir string) string {
	cmd := exec.Command("go", "list")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pkgpath, err := cmd.Output()
	if err != nil {
		msg := fmt.Sprintf("go list failed: %v - stdout: %v - stderr: %v", err, string(pkgpath), stderr.String())
		if strings.Contains(stderr.String(), "cannot find module providing package") {
			msg = "go list failed. Make sure you have .go files where your .proto file is." +
				"Also make sure to run the --go_out=. --twirp_out=. plugins on a separate command before you run --twirpql_out"
		}
		panic(msg)
	}
	return strings.TrimSpace(string(pkgpath))
}

func (tql *twirpql) setImportPath(serviceDir string) {
	modname := tql.goList(serviceDir)
	tql.modname = tql.Parameters().StrDefault("importpath", modname)
	if tql.modname == "" {
		panic("import path must be provided by `go list` in the .proto directory or through the importpath plugin parameter")
	}
}

func (tql *twirpql) generateSchema(f pgs.File, out io.Writer) {
	out.Write([]byte("# Code was generated by marwan.io/protoc-gen-twirpql. DO NOT EDIT.\n\n"))
	tql.svcname = tql.svc.Name().String()
	tql.gopkgname = tql.ctx.PackageName(f).String()
	gqlFile := &file{}
	gqlFile.Service = tql.getService(tql.svc)
	for _, v := range tql.inputs {
		gqlFile.Inputs = append(gqlFile.Inputs, v)
	}
	for _, v := range tql.types {
		gqlFile.Types = append(gqlFile.Types, v)
	}
	for k, v := range tql.enums {
		gqlFile.Enums = append(gqlFile.Enums, &enums{Name: k, Fields: v.Values})
	}
	for k := range tql.maps {
		gqlFile.Scalars = append(gqlFile.Scalars, k)
	}
	for _, v := range tql.unions {
		gqlFile.Unions = append(gqlFile.Unions, v)
	}

	var buf bytes.Buffer

	err := tql.tmpl.Execute(&buf, gqlFile)
	must(err)
	err = gqlfmt.Print(buf.String(), out)
	must(err)
}

// bridgeEnums creates a type conversion between
// protobuf's enums (int32) and GraphQL's enums (string).
func (tql *twirpql) bridgeEnums() {
	f, err := os.Create(tql.path("enums.gen.go"))
	must(err)
	defer f.Close()
	all := []*genenums.Data{}
	for k, v := range tql.enums {
		all = append(all, &genenums.Data{
			ImportPath: v.ImportPath,
			Pkg:        v.PackageName,
			Name:       k,
			GoName:     v.Name,
		})
	}
	must(genenums.Render(all, f))
}

func (tql *twirpql) touchConfig(out io.Writer) {
	out.Write([]byte("# Code was generated by marwan.io/protoc-gen-twirpql. DO NOT EDIT.\n\n"))
	var cfg gqlconfig.Config
	cfg.SchemaFilename = gqlconfig.StringList{tql.path("schema.graphql")}
	cfg.Exec = gqlconfig.PackageConfig{Filename: tql.path("generated.go")}
	cfg.Resolver = gqlconfig.PackageConfig{Filename: tql.path("resolver.go"), Type: "Resolver"}
	cfg.Models = tql.gqlTypes
	cfg.Model = gqlconfig.PackageConfig{Filename: tql.path("models_gen.go")}
	must(yaml.NewEncoder(out).Encode(&cfg))
}

func (tql *twirpql) initGql(svcName string) {
	cfg, err := gqlconfig.LoadConfig(tql.path("gqlgen.yml"))
	must(err)
	emptys := []string{}
	for k := range tql.emptys {
		emptys = append(emptys, k)
	}

	err = api.Generate(
		cfg,
		api.NoPlugins(),
		api.AddPlugin(modelgen.New()),
		api.AddPlugin(genresolver.New(svcName, tql.gopkgname, emptys, tql.maps, tql.unionNames)),
		api.AddPlugin(genserver.New(tql.path("server.go"), tql.modname, svcName)),
	)
	must(err)
}

func (tql *twirpql) getService(svc pgs.Service) *service {
	var s service
	s.Methods, s.Mutations = tql.getMethods(svc.Methods())
	return &s
}

func (tql *twirpql) getMethods(protoMethods []pgs.Method) ([]*method, []*method) {
	methods := []*method{}
	mutations := []*method{}

	// collect all types first, so that we de-dupe mixed
	// inputs && types
	for _, pm := range protoMethods {
		tql.setType(pm.Output())
	}

	for _, pm := range protoMethods {
		var m method
		m.Name = pm.Name().LowerCamelCase().String()
		// TODO: make oneOf fields a scalar in inputs
		emptyInput := len(pm.Input().NonOneOfFields()) == 0
		if !emptyInput {
			tql.setInput(pm.Input())
			m.Request = tql.formatQueryInput(pm.Input())
		}
		m.Response = tql.getQualifiedName(pm.Output())
		if tql.isMutation(pm) {
			mutations = append(mutations, &m)
		} else {
			methods = append(methods, &m)
		}
	}
	return methods, mutations
}

func (tql *twirpql) isMutation(pm pgs.Method) bool {
	opts := pm.Descriptor().GetOptions()
	if proto.HasExtension(opts, E_Modifiers) {
		mut, err := proto.GetExtension(opts, E_Modifiers)
		must(err)
		val, ok := mut.(*Modifiers)
		if !ok {
			panic(fmt.Sprintf("invalid mutation type: %T\n", mut))
		}
		return val.GetMutation()
	}
	return false
}

func (tql *twirpql) setType(msg pgs.Message) {
	typeName := tql.getQualifiedName(msg)
	if _, ok := tql.types[typeName]; ok {
		return
	}
	var i serviceType
	i.Name = typeName
	tql.types[i.Name] = &i
	tql.setGraphQLType(i.Name, msg)
	i.Fields = tql.getFields(msg.NonOneOfFields(), true)
	i.Fields = append(i.Fields, tql.getUnionFields(msg)...)
}

func (tql *twirpql) getUnionFields(msg pgs.Message) []*serviceField {
	sff := []*serviceField{}
	for _, oo := range msg.OneOfs() {
		unionTypes := []string{}
		unionName := tql.getUnionName(oo)
		for _, f := range oo.Fields() {
			tql.setUnionType(f) // side effect
			unionTypes = append(unionTypes, tql.getUnionFieldWrapperName(f))
		}
		// side effect
		tql.unionNames[oo.Name().UpperCamelCase().String()] = true
		tql.unions[unionName] = &union{
			Name:  unionName,
			Types: unionTypes,
		}
		importpath := tql.destimportpath + "/twirpql"
		tql.gqlTypes[tql.getUnionName(oo)] = gqlconfig.TypeMapEntry{
			Model: gqlconfig.StringList{importpath + "." + "unionMask"},
		}
		var sf serviceField
		sf.Name = oo.Name().String()
		sf.Type = tql.getUnionName(oo)
		sff = append(sff, &sf)
	}
	return sff
}

func (tql *twirpql) setUnionType(f pgs.Field) {
	typeName := tql.getUnionFieldWrapperName(f)
	if _, ok := tql.types[typeName]; ok {
		return
	}
	var i serviceType
	i.Name = typeName
	i.Fields = []*serviceField{tql.getField(f, true)}
	tql.types[i.Name] = &i
	// protoName might have unlimited trailing "_"s.
	// See: https://github.com/golang/protobuf/blob/master/protoc-gen-go/generator/generator.go#L2334
	protoName := f.Message().Name().String() + "_" + strings.Title(f.Name().String())
	tql.gqlTypes[i.Name] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{tql.deduceImportPath(f) + "." + protoName},
	}
}

func (tql *twirpql) getUnionFieldWrapperName(f pgs.Field) string {
	return tql.getUnionName(f.OneOf()) + f.Name().UpperCamelCase().String()
}

func (tql *twirpql) getUnionName(field pgs.OneOf) string {
	return tql.getQualifiedName(field.Message()) + field.Name().UpperCamelCase().String()
}

// getQualifiedName returns the name that will be defined inside the GraphQL Schema File.
// For messgae declarations that are part of the target .proto file, they will stay the same
// but if it's part of an import like "google.protobuf.Timestamp" then we combine the package name
// with the Message namd to ensure we have no clashes so it becomes: "google_protobuf_Timestamp"
func (tql *twirpql) getQualifiedName(msg pgs.Entity) string {
	msgGoTypeName := tql.ctx.Name(msg).String()
	if msg.Package() == tql.protopkg {
		return msgGoTypeName
	}
	pkgName := strings.ReplaceAll(msg.Package().ProtoName().String(), ".", "_")
	return strings.Title(pkgName + "_" + msgGoTypeName)
}

func (tql *twirpql) setInput(msg pgs.Message) {
	if _, ok := tql.inputs[tql.getInputName(msg)]; ok {
		return
	}
	var i serviceType
	i.Name = tql.getInputName(msg)
	tql.inputs[i.Name] = &i
	tql.setGraphQLType(i.Name, msg)
	// TODO: make oneOf fields scalars.
	i.Fields = tql.getFields(msg.NonOneOfFields(), false)
}

// getInputName returns exactly the name of the message declaration:
// message SomeMessage {
//   ... fields
// }
// would return SomeMessage. However, if SomeMessage was also
// used as an Output and not just Input, then GraphQL will
// not allow an Input and a Type to be the same name, therefore
// we will append an "Input" so that it becomes SomeMessageInput.
func (tql *twirpql) getInputName(msg pgs.Message) string {
	msgName := tql.getQualifiedName(msg)
	if _, ok := tql.types[msgName]; ok {
		return msgName + "Input"
	}
	return msgName
}

func (tql *twirpql) setGraphQLType(name string, msg pgs.Message) {
	if len(msg.Fields()) == 0 {
		tql.emptys[name] = true
		return
	}
	msgName := tql.ctx.Name(msg).String()
	importpath := tql.deduceImportPath(msg)
	tql.gqlTypes[name] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{importpath + "." + msgName},
	}
}

// deduceImportPath takes a protobuf message and does its best
// to tell you what the Go import path is for that message.
// At first, it checks if the go_package option is the same
// as the current working directory, if that's the case
// we already called "go list" and we just return tql.modname.
// Second, if the import path contains one or more "/" chars,
// then we return exactly the go_package option because this
// could mean the import path is somewhere outside of the .proto
// file such as "google.protobuf.Timestamp" pointing to
// "github.com/golang/protobuf/ptypes/timestamp".
// Last, assume the location of the .proto file is in a
// subdirectory from within the project, so we just call
// "go list" from within that subdirectory.
func (tql *twirpql) deduceImportPath(msg pgs.Entity) string {
	gopkg := tql.ctx.ImportPath(msg.File()).String()
	if gopkg == "." {
		return tql.modname
	}
	if strings.Contains(gopkg, "/") {
		return gopkg
	}

	return tql.goList(msg.File().InputPath().Dir().String())
}

func (tql *twirpql) setEnum(protoEnum pgs.Enum) {
	name := tql.getQualifiedName(protoEnum)
	if _, ok := tql.enums[name]; ok {
		return
	}
	vals := []string{}
	for _, v := range protoEnum.Values() {
		vals = append(vals, v.Name().String())
	}
	tql.enums[name] = &enumData{
		Name:        tql.ctx.Name(protoEnum).String(),
		ImportPath:  tql.deduceImportPath(protoEnum),
		PackageName: tql.ctx.PackageName(protoEnum.File()).String(),
		Values:      vals,
	}
	tql.setGraphQLEnum(name, protoEnum)
}

func (tql *twirpql) setGraphQLEnum(name string, enum pgs.Enum) {
	importpath := tql.deduceImportPath(enum)
	enumGoTypeName := tql.ctx.Name(enum).String()
	tql.gqlTypes[name] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{importpath + "." + enumGoTypeName},
	}
}

func (tql *twirpql) setBytes(fieldName string, f pgs.Field) {
	tql.maps[fieldName] = tql.ctx.Type(f).Value().String()
	tql.gqlTypes[fieldName] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{tql.destimportpath + "/twirpql." + fieldName},
	}
}

func (tql *twirpql) setMap(fieldName string, f pgs.Field) {
	upField := strings.Title(fieldName)
	switch f.Type().Element().ProtoType().Proto() {
	case 11:
		mapValue := f.Type().Element().Embed()
		tql.mapImports[tql.deduceImportPath(mapValue)] = struct{}{}
		goTypeDeclaration := strings.ReplaceAll(
			tql.ctx.Type(f).Value().String(),
			mapValue.Name().String(),
			tql.ctx.PackageName(mapValue).String()+"."+
				mapValue.Name().String(),
		)
		tql.maps[upField] = goTypeDeclaration
	case 14:
		mapValue := f.Type().Element().Enum()
		tql.mapImports[tql.deduceImportPath(mapValue)] = struct{}{}
		goTypeDeclaration := strings.ReplaceAll(
			tql.ctx.Type(f).Value().String(),
			mapValue.Name().String(),
			tql.ctx.PackageName(mapValue).String()+"."+
				mapValue.Name().String(),
		)
		tql.maps[upField] = goTypeDeclaration
	default:
		tql.maps[upField] = tql.ctx.Type(f).Value().String()
	}
	tql.gqlTypes[upField] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{tql.destimportpath + "/twirpql." + upField},
	}
}

func (tql *twirpql) getFields(protoFields []pgs.Field, isType bool) []*serviceField {
	fields := []*serviceField{}
	for _, pf := range protoFields {
		fields = append(fields, tql.getField(pf, isType))
	}
	return fields
}

func (tql *twirpql) writeUnionMask() {
	f, err := os.Create(filepath.Join(tql.destpkgname, "unions.gen.go"))
	must(err)
	defer f.Close()
	err = genunions.Render(f)
	must(err)
}

func (tql *twirpql) getField(pf pgs.Field, isType bool) *serviceField {
	var f serviceField
	f.Name = pf.Name().String()
	pt := pf.Type().ProtoType().Proto()
	var tmp string
	switch pt {
	case 11:
		if pf.Type().IsMap() {
			tql.setMap(f.Name, pf)
			tmp = strings.Title(f.Name)
		} else {
			var msg pgs.Message
			if pf.Type().IsRepeated() {
				msg = pf.Type().Element().Embed()
			} else {
				msg = pf.Type().Embed()
			}
			if isType {
				tmp = tql.getQualifiedName(msg)
				tql.setType(msg)
			} else {
				tmp = tql.getInputName(msg)
				tql.setInput(msg)
			}
		}
	case 14:
		tql.setEnum(pf.Type().Enum())
		tmp = tql.getQualifiedName(pf.Type().Enum())
	case 12:
		tmp = strings.Title(f.Name)
		tql.setBytes(tmp, pf)
	default:
		tmp = protoTypesToGqlTypes[pt.String()]
		if tmp == "" {
			panic("unsupported type: " + pt.String())
		}
	}
	if pf.Type().IsRepeated() {
		tmp = fmt.Sprintf("[%v]", tmp)
	}
	f.Type = tmp
	return &f
}

// formatQueryInput returns a template-formatted representation
// of a query input. In GraphQL a query looks like this:
// `someQuery(req: Request): Response`
// However, if we don't want to have an input at all in a query,
// the query will now have to look like this:
// `someQuery: Response`
func (tql *twirpql) formatQueryInput(msg pgs.Message) string {
	return fmt.Sprintf("(req: %v)", tql.getInputName(msg))
}

func (tql *twirpql) path(s string) string {
	return filepath.Join(tql.destpkgname, s)
}

var protoTypesToGqlTypes = map[string]string{
	"TYPE_DOUBLE":  "Float",
	"TYPE_FLOAT":   "Float",
	"TYPE_INT64":   "Int",
	"TYPE_UINT64":  "Int",
	"TYPE_INT32":   "Int",
	"TYPE_FIXED64": "Float",
	"TYPE_FIXED32": "Float",
	"TYPE_BOOL":    "Boolean",
	"TYPE_STRING":  "String",
	// "TYPE_GROUP": "",
	// "TYPE_MESSAGE": "", // must be mapped to its sibling type
	// "TYPE_BYTES":  "",
	"TYPE_UINT32": "Int",
	// "TYPE_ENUM": "", // mapped to its sibling type
	// "TYPE_SFIXED32": "",
	// "TYPE_SFIXED64": "",
	// "TYPE_SINT32": "",
	// "TYPE_SINT64": "",
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
