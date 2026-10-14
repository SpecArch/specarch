package sourcemap

import "github.com/SpecArch/specarch/third_party/typescript-go/core"

type Source interface {
	Text() string
	FileName() string
	ECMALineMap() []core.TextPos
}
