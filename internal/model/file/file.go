package file

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusUploaded FileStatus = "uploaded"
	FileStatusFailed   FileStatus = "failed"
	FileStatusDeleted  FileStatus = "deleted"
)

type File struct {
	ID           uuid.UUID      `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID       uuid.UUID      `json:"userId" gorm:"type:uuid;not null;index"`
	BucketName   string         `json:"bucketName" gorm:"not null"`
	ObjectKey    string         `json:"objectKey" gorm:"not null;uniqueIndex"`
	ETag         string         `json:"etag"`
	OriginalName string         `json:"originalName" gorm:"not null"`
	ContentType  string         `json:"contentType" gorm:"not null"`
	SizeBytes    int64          `json:"sizeBytes" gorm:"not null"`
	Width        int            `json:"width,omitempty"`
	Height       int            `json:"height,omitempty"`
	Status       FileStatus     `json:"status" gorm:"type:varchar(20);default:'pending'"`
	IsPublic     bool           `json:"isPublic" gorm:"default:false"`
	Checksum     string         `json:"checksum,omitempty"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
}

func (f *File) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
