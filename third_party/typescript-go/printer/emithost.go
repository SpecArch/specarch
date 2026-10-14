package printer

import (
	"github.com/SpecArch/specarch/third_party/typescript-go/ast"
	"github.com/SpecArch/specarch/third_party/typescript-go/core"
	"github.com/SpecArch/specarch/third_party/typescript-go/tsoptions"
	"github.com/SpecArch/specarch/third_party/typescript-go/tspath"
)

// NOTE: EmitHost operations must be thread-safe
type EmitHost interface {
	Options() *core.CompilerOptions
	SourceFiles() []*ast.SourceFile
	UseCaseSensitiveFileNames() bool
	GetCurrentDirectory() string
	CommonSourceDirectory() string
	IsEmitBlocked(file string) bool
	WriteFile(fileName string, text string) error
	GetEmitModuleFormatOfFile(file ast.HasFileName) core.ModuleKind
	GetEmitResolver() EmitResolver
	GetProjectReferenceFromSource(path tspath.Path) *tsoptions.SourceOutputAndProjectReference
	IsSourceFileFromExternalLibrary(file *ast.SourceFile) bool
}
