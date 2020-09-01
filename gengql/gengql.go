package gengql

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
	"github.com/tmc/protoc-gen-gql/internal/genenums"
	"github.com/tmc/protoc-gen-gql/internal/genresolver"
	"github.com/tmc/protoc-gen-gql/internal/genserver"
	"github.com/tmc/protoc-gen-gql/internal/genunions"
	"github.com/tmc/protoc-gen-gql/internal/gqlfmt"
	"github.com/tmc/protoc-gen-gql/options"
	"gopkg.in/yaml.v2"
)

// gengql creates a report of all the target messages generated by the
// protoc run, writing the file into the /tmp directory.
type gengql struct {
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

	// responseUnions represent the name
	// of all the RPCs that want their
	// responses combined with an error type
	// This way, the resolver can replace
	// a response with the error type
	// instead of actually returning the error.
	responseUnions map[string]string

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

	sdl string

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
	// live. It defaults to a "gengql".
	destpkgname string

	// is the import path that will import
	// the gengql sub-package
	destimportpath string

	svc      pgs.Service
	protopkg pgs.Package
}

type enumData struct {
	Name        string
	ImportPath  string
	PackageName string
	Values      []*serviceField
	Doc         string
}

// New configures the module with an instance of ModuleBase
func New(importPath string) pgs.Module {
	return &gengql{
		ModuleBase:     &pgs.ModuleBase{},
		inputs:         map[string]*serviceType{},
		types:          map[string]*serviceType{},
		emptys:         map[string]bool{},
		enums:          map[string]*enumData{},
		maps:           map[string]string{},
		mapImports:     map[string]struct{}{},
		unions:         map[string]*union{},
		unionNames:     map[string]bool{},
		responseUnions: map[string]string{},
		gqlTypes:       gqlconfig.TypeMap{},
		tmpl:           template.Must(template.New("").Funcs(schemaFuncs).Parse(schemaTemplate)),
		modname:        importPath,
		ctx:            pgsgo.InitContext(pgs.ParseParameters("")),
		destpkgname:    "./gengql",
		destimportpath: "",
	}
}

// Name is the identifier used to identify the module. This value is
// automatically attached to the BuildContext associated with the ModuleBase.
func (tql *gengql) Name() string { return "gengql" }

func (tql *gengql) InitContext(c pgs.BuildContext) {
	tql.ModuleBase.InitContext(c)
	tql.ctx = pgsgo.InitContext(c.Parameters())
}

// Execute is passed the target files as well as its dependencies in the pkgs
// map. The implementation should return a slice of Artifacts that represent
// the files to be generated. In this case, "/tmp/report.txt" will be created
// outside of the normal protoc flow.
func (tql *gengql) Execute(targets map[string]pgs.File, pkgs map[string]pgs.Package) []pgs.Artifact {
	tql.destpkgname = tql.Parameters().StrDefault("dest", tql.destpkgname)
	os.MkdirAll(tql.destpkgname, 0777)

	if len(targets) != 1 {
		panic("only one proto file is supported at this moment; see https://gengql.dev/docs/multiple-services")
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
		var schemaBuffer bytes.Buffer
		f, err := os.Create(tql.path("schema.graphql"))
		must(err)
		defer f.Close()
		tql.generateSchema(targetFile, io.MultiWriter(&schemaBuffer, f))
		if tql.isFederated(targetFile) {
			tql.sdl = strings.Replace(schemaBuffer.String(), "type Query", "extend type Query", 1)
		}
	}
	return tql.Artifacts()
}

func (tql *gengql) pickServiceFromFile(svc string, f pgs.File) pgs.Service {
	switch len(f.Services()) {
	case 0:
		panic("proto file must have at least one service")
	case 1:
		return f.Services()[0]
	}
	if svc == "" {
		panic("service name must be provided if proto file has multiple services; see https://gengql.dev/docs/multiple-services")
	}
	for _, service := range f.Services() {
		if svc == service.Name().String() {
			return service
		}
	}
	panic("protofile does not have the given service: " + svc)
}

func (tql *gengql) goList(dir string) string {
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
				"Also make sure to run the --go_out=. --gql_out=. plugins on a separate command before you run --gengql_out"
		}
		panic(msg)
	}
	return strings.TrimSpace(string(pkgpath))
}

func (tql *gengql) setImportPath(serviceDir string) {
	modname := tql.goList(serviceDir)
	tql.modname = tql.Parameters().StrDefault("importpath", modname)
	if tql.modname == "" {
		panic("import path must be provided by `go list` in the .proto directory or through the importpath plugin parameter")
	}
}

func (tql *gengql) generateSchema(f pgs.File, out io.Writer) {
	out.Write([]byte("# Code was generated by github.com/tmc/protoc-gen-gql. DO NOT EDIT.\n\n"))
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
		gqlFile.Enums = append(gqlFile.Enums, &enums{
			Name:   k,
			Fields: v.Values,
			Doc:    v.Doc,
		})
	}
	for k := range tql.maps {
		gqlFile.Scalars = append(gqlFile.Scalars, k)
	}
	for _, v := range tql.unions {
		gqlFile.Unions = append(gqlFile.Unions, v)
	}
	if tql.isFederated(f) {
		gqlFile.Service.Methods = append(gqlFile.Service.Methods, &method{
			Name:     "_service",
			Request:  "",
			Response: "_Service",
		})
		gqlFile.Types = append(gqlFile.Types, &serviceType{
			Name: "_Service",
			Fields: []*serviceField{&serviceField{
				Name: "sdl",
				Type: "String",
			}},
		})
	}

	var buf bytes.Buffer

	err := tql.tmpl.Execute(&buf, gqlFile)
	must(err)
	err = gqlfmt.Print(buf.String(), out)
	must(err)
}

// bridgeEnums creates a type conversion between
// protobuf's enums (int32) and GraphQL's enums (string).
func (tql *gengql) bridgeEnums() {
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

func (tql *gengql) touchConfig(out io.Writer) {
	out.Write([]byte("# Code was generated by github.com/tmc/protoc-gen-gql. DO NOT EDIT.\n\n"))
	var cfg gqlconfig.Config
	cfg.SchemaFilename = gqlconfig.StringList{tql.path("schema.graphql")}
	cfg.Exec = gqlconfig.PackageConfig{Filename: tql.path("generated.go")}
	cfg.Resolver = gqlconfig.ResolverConfig{Filename: tql.path("resolver.go"), Type: "Resolver"}
	cfg.Models = tql.gqlTypes
	cfg.Model = gqlconfig.PackageConfig{Filename: tql.path("models_gen.go")}
	must(yaml.NewEncoder(out).Encode(&cfg))
}

func (tql *gengql) initGql(svcName string) {
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
		api.AddPlugin(genresolver.New(
			svcName,
			tql.gopkgname,
			emptys,
			tql.maps,
			tql.unionNames,
			tql.responseUnions,
			tql.sdl,
		)),
		api.AddPlugin(genserver.New(tql.path("server.go"), tql.modname, svcName)),
	)
	must(err)
}

func (tql *gengql) getService(svc pgs.Service) *service {
	var s service
	s.Methods, s.Mutations = tql.getMethods(svc.Methods())
	return &s
}

func (tql *gengql) getMethods(protoMethods []pgs.Method) ([]*method, []*method) {
	methods := []*method{}
	mutations := []*method{}

	// collect all types first, so that we de-dupe mixed
	// inputs && types
	for _, pm := range protoMethods {
		tql.setType(pm.Output())
	}

	for _, pm := range protoMethods {
		if tql.isSkipped(pm) {
			continue
		}
		var m method
		m.Name = pm.Name().LowerCamelCase().String()
		m.Doc = pm.SourceCodeInfo().LeadingComments()
		// TODO: make oneOf fields a scalar in inputs
		emptyInput := len(pm.Input().NonOneOfFields()) == 0
		if !emptyInput {
			tql.setInput(pm.Input())
			m.Request = tql.formatQueryInput(pm.Input())
		}
		if tql.hasResponseCombination(pm) {
			m.Response = tql.setResponseCombination(pm)
		} else {
			m.Response = tql.getQualifiedName(pm.Output())
		}
		if tql.isMutation(pm) {
			mutations = append(mutations, &m)
		} else {
			methods = append(methods, &m)
		}
	}
	return methods, mutations
}

func (tql *gengql) setResponseCombination(m pgs.Method) string {
	rpc := getModifiers(m)
	typeName := rpc.GetRespondsWith()[0]
	f := m.File()
	var msg pgs.Message
	for _, m := range f.Messages() {
		if typeName == m.Name().String() {
			msg = m
		}
	}
	if msg == nil {
		panic(typeName + " is not defined in proto file")
	}
	tql.setType(msg)
	responseName := tql.getQualifiedName(m.Output())
	unionName := responseName + "Set"
	tql.unions[unionName] = &union{
		Name:  unionName,
		Types: []string{responseName, typeName},
	}
	tql.responseUnions[m.Name().UpperCamelCase().String()] = typeName
	importpath := tql.destimportpath + "/gengql"
	tql.gqlTypes[unionName] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{importpath + "." + "unionMask"},
	}
	return unionName
}

func (tql *gengql) hasResponseCombination(m pgs.Method) bool {
	rpc := getModifiers(m)
	return len(rpc.GetRespondsWith()) > 0
}

func (tql *gengql) isMutation(pm pgs.Method) bool {
	val := getModifiers(pm)
	return val.GetMutation()
}

func (tql *gengql) isFederated(f pgs.File) bool {
	opts := f.Descriptor().GetOptions()
	if proto.HasExtension(opts, options.E_Schema) {
		mut, err := proto.GetExtension(opts, options.E_Schema)
		must(err)
		val, ok := mut.(*options.Schema)
		if !ok {
			panic(fmt.Sprintf("invalid mutation type: %T\n", mut))
		}
		return val.GetFederated()
	}
	return false
}

func (tql *gengql) isSkipped(pm pgs.Method) bool {
	val := getModifiers(pm)
	return val.GetSkip()
}

func getModifiers(pm pgs.Method) *options.RPC {
	opts := pm.Descriptor().GetOptions()
	if proto.HasExtension(opts, options.E_Rpc) {
		rpc, err := proto.GetExtension(opts, options.E_Rpc)
		must(err)
		val, ok := rpc.(*options.RPC)
		if !ok {
			panic(fmt.Sprintf("invalid rpc type: %T\n", rpc))
		}
		return val
	}
	return nil
}

func (tql *gengql) setType(msg pgs.Message) {
	typeName := tql.getQualifiedName(msg)
	if _, ok := tql.types[typeName]; ok {
		return
	}
	var i serviceType
	i.Name = typeName
	i.Doc = msg.SourceCodeInfo().LeadingComments()
	tql.types[i.Name] = &i
	tql.setGraphQLType(i.Name, msg)
	i.Fields = tql.getFields(msg.NonOneOfFields(), true)
	i.Fields = append(i.Fields, tql.getUnionFields(msg)...)
}

func (tql *gengql) getUnionFields(msg pgs.Message) []*serviceField {
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
		importpath := tql.destimportpath + "/gengql"
		tql.gqlTypes[unionName] = gqlconfig.TypeMapEntry{
			Model: gqlconfig.StringList{importpath + "." + "unionMask"},
		}
		var sf serviceField
		sf.Name = oo.Name().String()
		sf.Type = unionName
		sff = append(sff, &sf)
	}
	return sff
}

func (tql *gengql) setUnionType(f pgs.Field) {
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

func (tql *gengql) getUnionFieldWrapperName(f pgs.Field) string {
	return tql.getUnionName(f.OneOf()) + f.Name().UpperCamelCase().String()
}

func (tql *gengql) getUnionName(field pgs.OneOf) string {
	return tql.getQualifiedName(field.Message()) + field.Name().UpperCamelCase().String()
}

// getQualifiedName returns the name that will be defined inside the GraphQL Schema File.
// For messgae declarations that are part of the target .proto file, they will stay the same
// but if it's part of an import like "google.protobuf.Timestamp" then we combine the package name
// with the Message namd to ensure we have no clashes so it becomes: "google_protobuf_Timestamp"
func (tql *gengql) getQualifiedName(msg pgs.Entity) string {
	msgGoTypeName := tql.ctx.Name(msg).String()
	if msg.Package() == tql.protopkg {
		return msgGoTypeName
	}
	pkgName := strings.ReplaceAll(msg.Package().ProtoName().String(), ".", "_")
	return strings.Title(pkgName + "_" + msgGoTypeName)
}

func (tql *gengql) setInput(msg pgs.Message) {
	if _, ok := tql.inputs[tql.getInputName(msg)]; ok {
		return
	}
	var i serviceType
	i.Name = tql.getInputName(msg)
	i.Doc = msg.SourceCodeInfo().LeadingComments()
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
func (tql *gengql) getInputName(msg pgs.Message) string {
	msgName := tql.getQualifiedName(msg)
	if _, ok := tql.types[msgName]; ok {
		return msgName + "Input"
	}
	return msgName
}

func (tql *gengql) setGraphQLType(name string, msg pgs.Message) {
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
func (tql *gengql) deduceImportPath(msg pgs.Entity) string {
	gopkg := tql.ctx.ImportPath(msg.File()).String()
	if gopkg == "." {
		return tql.modname
	}
	if strings.Contains(gopkg, "/") {
		return gopkg
	}

	return tql.goList(msg.File().InputPath().Dir().String())
}

func (tql *gengql) setEnum(protoEnum pgs.Enum) {
	name := tql.getQualifiedName(protoEnum)
	if _, ok := tql.enums[name]; ok {
		return
	}
	vals := []*serviceField{}
	for _, v := range protoEnum.Values() {
		vals = append(vals, &serviceField{
			Name: v.Name().String(),
			Doc:  v.SourceCodeInfo().LeadingComments(),
		})
	}
	tql.enums[name] = &enumData{
		Name:        tql.ctx.Name(protoEnum).String(),
		Doc:         protoEnum.SourceCodeInfo().LeadingComments(),
		ImportPath:  tql.deduceImportPath(protoEnum),
		PackageName: tql.ctx.PackageName(protoEnum.File()).String(),
		Values:      vals,
	}
	tql.setGraphQLEnum(name, protoEnum)
}

func (tql *gengql) setGraphQLEnum(name string, enum pgs.Enum) {
	importpath := tql.deduceImportPath(enum)
	enumGoTypeName := tql.ctx.Name(enum).String()
	tql.gqlTypes[name] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{importpath + "." + enumGoTypeName},
	}
}

func (tql *gengql) setBytes(fieldName string, f pgs.Field) {
	tql.maps["ProtoBytes"] = tql.ctx.Type(f).Value().String()
	tql.gqlTypes["ProtoBytes"] = gqlconfig.TypeMapEntry{
		Model: gqlconfig.StringList{tql.destimportpath + "/gengql." + "ProtoBytes"},
	}
}

func (tql *gengql) setMap(fieldName string, f pgs.Field) {
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
		Model: gqlconfig.StringList{tql.destimportpath + "/gengql." + upField},
	}
}

func (tql *gengql) getFields(protoFields []pgs.Field, isType bool) []*serviceField {
	fields := []*serviceField{}
	for _, pf := range protoFields {
		fields = append(fields, tql.getField(pf, isType))
	}
	return fields
}

func (tql *gengql) writeUnionMask() {
	f, err := os.Create(filepath.Join(tql.destpkgname, "unions.gen.go"))
	must(err)
	defer f.Close()
	err = genunions.Render(f)
	must(err)
}

func (tql *gengql) getField(pf pgs.Field, isType bool) *serviceField {
	var f serviceField
	f.Name = pf.Name().String()
	f.Doc = pf.SourceCodeInfo().LeadingComments()
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
		e := pf.Type().Enum()
		if pf.Type().IsRepeated() {
			e = pf.Type().Element().Enum()
		}
		tql.setEnum(e)
		tmp = tql.getQualifiedName(e)
	case 12:
		tmp = "ProtoBytes"
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
func (tql *gengql) formatQueryInput(msg pgs.Message) string {
	return fmt.Sprintf("(req: %v)", tql.getInputName(msg))
}

func (tql *gengql) path(s string) string {
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
