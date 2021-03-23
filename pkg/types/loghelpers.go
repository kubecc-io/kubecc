/*
Copyright 2021 The Kubecc Authors.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

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
