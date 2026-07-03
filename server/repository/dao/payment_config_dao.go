package dao

import (
	"context"
	"sync"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

type PaymentConfigDao interface {
	ListAll(ctx context.Context) ([]*model.PaymentConfig, error)
	FindExistingPayAddresses(ctx context.Context, payAddresses []string) (map[string]struct{}, error)
}

type PaymentConfigDaoImpl struct {
	db *gorm.DB
}

var (
	paymentConfigOnce sync.Once
	paymentConfigDao  PaymentConfigDao
)

func GetPaymentConfigDao() PaymentConfigDao {
	paymentConfigOnce.Do(func() {
		paymentConfigDao = &PaymentConfigDaoImpl{db: repository.DB}
	})
	return paymentConfigDao
}

func (d *PaymentConfigDaoImpl) ListAll(ctx context.Context) ([]*model.PaymentConfig, error) {
	var configs []*model.PaymentConfig
	if err := d.db.WithContext(ctx).Order("id ASC").Find(&configs).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to list payment configs: %v", err)
		return nil, err
	}
	return configs, nil
}

func (d *PaymentConfigDaoImpl) FindExistingPayAddresses(ctx context.Context, payAddresses []string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	if len(payAddresses) == 0 {
		return result, nil
	}

	unique := make([]string, 0, len(payAddresses))
	seen := make(map[string]struct{}, len(payAddresses))
	for _, addr := range payAddresses {
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		unique = append(unique, addr)
	}

	var found []string
	err := d.db.WithContext(ctx).Model(&model.PaymentConfig{}).
		Where("pay_address IN ?", unique).
		Pluck("pay_address", &found).Error
	if err != nil {
		log.WithContext(ctx).Errorf("failed to batch check payment configs: %v", err)
		return nil, err
	}
	for _, addr := range found {
		result[addr] = struct{}{}
	}
	return result, nil
}
