package postgres

import (
	"github.com/astianmuchui/mobilesonar/internal/models"
	"gorm.io/gorm"
)

func GetUserByEmail(db *gorm.DB, email string) (*models.User, error) {
	var user models.User
	err := db.Where("email = ?", email).First(&user).Error
	return &user, err
}

func CreateUser(db *gorm.DB, user *models.User) error {
	return db.Create(user).Error
}
