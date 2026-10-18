package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// StudioWorkspace is one account-scoped snapshot. Manifest remains opaque to
// preserve client-owned canvas fields across server upgrades.
type StudioWorkspace struct {
	UserID    int    `gorm:"primaryKey;autoIncrement:false"`
	Revision  int64  `gorm:"not null"`
	Manifest  string `gorm:"type:text;not null"`
	UpdatedAt int64  `gorm:"not null"`
}

type StudioWorkspaceFile struct {
	ID          int    `gorm:"primaryKey"`
	UserID      int    `gorm:"uniqueIndex:idx_studio_workspace_file"`
	KeyHash     string `gorm:"type:varchar(64);uniqueIndex:idx_studio_workspace_file"`
	ContentHash string `gorm:"type:varchar(64);not null"`
	ContentType string `gorm:"type:varchar(191);not null"`
	CreatedAt   int64  `gorm:"not null"`
}

func FindStudioWorkspace(userID int) (*StudioWorkspace, error) {
	var workspace StudioWorkspace
	err := DB.Where("user_id = ?", userID).First(&workspace).Error
	return &workspace, err
}

func SaveStudioWorkspace(userID int, baseRevision int64, manifest string) (*StudioWorkspace, bool, error) {
	now := common.GetTimestamp()
	if baseRevision == 0 {
		workspace := &StudioWorkspace{UserID: userID, Revision: 1, Manifest: manifest, UpdatedAt: now}
		if err := DB.Create(workspace).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return nil, false, nil
			}
			existing, lookupErr := FindStudioWorkspace(userID)
			if lookupErr == nil {
				return existing, false, nil
			}
			return nil, false, err
		}
		return workspace, true, nil
	}

	result := DB.Model(&StudioWorkspace{}).Where("user_id = ? AND revision = ?", userID, baseRevision).
		Updates(map[string]any{"revision": baseRevision + 1, "manifest": manifest, "updated_at": now})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &StudioWorkspace{UserID: userID, Revision: baseRevision + 1, Manifest: manifest, UpdatedAt: now}, true, nil
}

func FindStudioWorkspaceFile(userID int, keyHash string) (*StudioWorkspaceFile, error) {
	var file StudioWorkspaceFile
	err := DB.Where("user_id = ? AND key_hash = ?", userID, keyHash).First(&file).Error
	return &file, err
}

func CreateStudioWorkspaceFile(file *StudioWorkspaceFile) (*StudioWorkspaceFile, bool, error) {
	if err := DB.Create(file).Error; err != nil {
		existing, lookupErr := FindStudioWorkspaceFile(file.UserID, file.KeyHash)
		if lookupErr == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return file, true, nil
}
