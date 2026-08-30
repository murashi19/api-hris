package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
	"hris/backend/internal/model"
)

func Write(ctx context.Context, db *gorm.DB, actorUserID *uint64, action, entityType string, entityID any, metadata any) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	entry := model.AuditLog{
		ActorUserID: actorUserID,
		Action:      action,
		EntityType:  entityType,
		EntityID:    fmt.Sprint(entityID),
		Metadata:    payload,
	}
	return db.WithContext(ctx).Create(&entry).Error
}
