package main

import (
	"fmt"
	"os"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: status-image-token PRIVATE_OUTPUT_FILE")
	}
	db, err := gorm.Open(postgres.Open(os.Getenv("SQL_DSN")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		panic("cannot connect to configured database")
	}
	model.DB = db
	var owner model.User
	if err := db.Select("id").Where("role = ? AND status = ?", common.RoleRootUser, common.UserStatusEnabled).First(&owner).Error; err != nil {
		panic("no active owner")
	}
	token := model.Token{}
	err = db.Where("name = ? AND user_id = ?", "status-image-bot-readonly", owner.Id).First(&token).Error
	if err == gorm.ErrRecordNotFound {
		key, keyErr := common.GenerateRandomKey(48)
		if keyErr != nil {
			panic("cannot generate query credential")
		}
		token = model.Token{
			UserId: owner.Id, Key: key, Name: "status-image-bot-readonly",
			Status: common.TokenStatusEnabled, ExpiredTime: -1, CreatedTime: time.Now().Unix(),
			RemainQuota: 0, ModelLimitsEnabled: true, ModelLimits: "__status_image_readonly__",
		}
		if err = token.Insert(); err != nil {
			panic("cannot create read-only query token")
		}
	} else if err != nil {
		panic("cannot inspect query token")
	}
	if token.RemainQuota != 0 || token.UnlimitedQuota || !token.ModelLimitsEnabled || token.ModelLimits != "__status_image_readonly__" || token.Status != common.TokenStatusEnabled {
		panic("existing token differs; no credential changed")
	}
	file, err := os.OpenFile(os.Args[1], os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		panic("private output exists or cannot be created")
	}
	if _, err := file.WriteString("sk-" + token.Key + "\n"); err != nil {
		panic("cannot write private credential")
	}
	if err := file.Close(); err != nil {
		panic("cannot finish private credential")
	}
	fmt.Printf("query_token_id=%d quota=0 model_access=disabled\n", token.Id)
}
