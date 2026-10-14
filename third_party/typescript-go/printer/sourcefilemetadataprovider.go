package printer

import (
	"github.com/SpecArch/specarch/third_party/typescript-go/ast"
	"github.com/SpecArch/specarch/third_party/typescript-go/tspath"
)

type SourceFileMetaDataProvider interface {
	GetSourceFileMetaData(path tspath.Path) *ast.SourceFileMetaData
}
