package repositories

import (
	"context"
	"errors"

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
	GetUserBySlackUserID(ctx context.Context, slackUserID string) (domain.User, error)
	SearchUsers(ctx context.Context) ([]domain.User, error)
}

func NewRepository(db *gorm.DB) Repositories {
	return repositories{
		DB: db,
	}
}

func (repo repositories) CreateUser(ctx context.Context, domainUser domain.User) error {
	user := db_entities.NewUserFromDomain(domainUser)
	return repo.DB.Create(&user).Error
}

func (repo repositories) GetUserBySlackUserID(ctx context.Context, slackUserID string) (domain.User, error) {
	user := db_entities.User{}
	if err := repo.DB.Take(&user, "slack_user_id = ?", slackUserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.User{}, app_error.UserNotFound
		}
		return domain.User{}, err
	}
	return user.ToDomain(), nil
}

func (repo repositories) SearchUsers(ctx context.Context) ([]domain.User, error) {
	users := db_entities.Users{}
	if err := repo.DB.Find(&users).Error; err != nil {
		return []domain.User{}, err
	}
	return users.ToDomain(), nil
}
