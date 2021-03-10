package types

import (
	"github.com/google/uuid"
	"go.uber.org/zap/zapcore"
)

func ShortID(uuid string) zapcore.Field {
	return zapcore.Field{
		Key:    "id",
		Type:   zapcore.StringType,
		String: FormatShortID(uuid, 6, ElideCenter),
	}
}

type ElideLocation int

const (
	ElideCenter ElideLocation = iota
	ElideLeft
	ElideRight
)

func FormatShortID(id string, length int, elide ElideLocation) string {
	if _, err := uuid.Parse(id); err != nil {
		// Not a UUID
		return id
	}
	switch elide {
	case ElideCenter:
		return id[:length/2] + ".." + id[len(id)-length/2:]
	case ElideLeft:
		return ".." + id[len(id)-length+1:]
	case ElideRight:
		return id[:length] + ".."
	}
	return id
}
