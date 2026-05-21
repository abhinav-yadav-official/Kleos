package resume

import "time"

type Record struct {
	ID          string
	UserID      string
	Filename    string
	StoragePath string
	ParsedText  string
	IsActive    bool
	CreatedAt   time.Time
}

type CreateInput struct {
	Filename    string
	ContentType string
	Data        []byte
}
