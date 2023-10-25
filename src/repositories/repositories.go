package repositories

import (
	"context"
	"fmt"

	"github.com/o-mago/spotify-status/src/app_error"
	"github.com/o-mago/spotify-status/src/domain"
	"github.com/o-mago/spotify-status/src/repositories/db_entities"
	"gorm.io/gorm"
)

type repositories struct {
	DB *gorm.DB
}

type Repositories interface {
	CreateUser(ctx context.Context, domainUser domain.User) error
	SearchUsers(ctx context.Context) ([]domain.User, error)
	UpdateUserEnabledBySlackID(ctx context.Context, domainUser domain.User) error
	RemoveUserBySlackID(ctx context.Context, slackID string) error
}

func NewRepository(db *gorm.DB) Repositories {
	return repositories{
		DB: db,
	}
}

func (repo repositories) CreateUser(ctx context.Context, domainUser domain.User) error {
	user := db_entities.NewUserFromDomain(domainUser)
	result := repo.DB.FirstOrCreate(&user)
	if result.Error != nil {
		fmt.Println(result.Statement)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return app_error.UserAlreadyExists
	}
	return nil
}

func (repo repositories) SearchUsers(ctx context.Context) ([]domain.User, error) {
	users := db_entities.Users{}
	if err := repo.DB.Where("enabled = ?", true).Find(&users).Error; err != nil {
		return []domain.User{}, err
	}
	return users.ToDomain(), nil
}

func (repo repositories) UpdateUserEnabledBySlackID(ctx context.Context, domainUser domain.User) error {
	user := db_entities.NewUserFromDomain(domainUser)
	result := repo.DB.Model(&db_entities.User{}).Where("slack_user_id = ?", user.SlackUserID).Update("enabled", user.Enabled)
	if result.Error != nil {
		fmt.Println(result.Statement)
		return result.Error
	}

	return nil
}

func (repo repositories) RemoveUserBySlackID(ctx context.Context, slackID string) error {
	result := repo.DB.Where("slack_user_id = ?", slackID).Exec("DELETE FROM users")
	if result.Error != nil {
		fmt.Println(result.Statement)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return app_error.RemoveUserError
	}
	return nil
}
