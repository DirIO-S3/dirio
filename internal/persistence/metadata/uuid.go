package metadata

import (
	"fmt"
	"uuid"
)

func FromBytesToUUID(b []byte) (uuid.UUID, error) {
	if len(b) != 16 {
		return uuid.UUID{}, fmt.Errorf("invalid UUID binary length: %d", len(b))
	}

	return uuid.UUID(b), nil
}
