package dao

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nusiss-capstone-project/reward-mservice/server/log"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/cache"
	"github.com/nusiss-capstone-project/reward-mservice/server/repository/model"
	"gorm.io/gorm"
)

const (
	paymentConfigCacheTTL     = 5 * time.Minute
	paymentConfigCacheCleanup = 10 * time.Minute
	paymentConfigCacheKeyFmt  = "payment_config:%s"
)

type PaymentConfigDao interface {
	ListAll(ctx context.Context) ([]*model.PaymentConfig, error)
	GetByPayAddress(ctx context.Context, payAddress string) (*model.PaymentConfig, error)
}

type PaymentConfigDaoImpl struct {
	db    *gorm.DB
	cache *cache.Cache[*model.PaymentConfig]
}

var (
	paymentConfigOnce sync.Once
	paymentConfigDao  PaymentConfigDao
)

func GetPaymentConfigDao() PaymentConfigDao {
	paymentConfigOnce.Do(func() {
		paymentConfigDao = &PaymentConfigDaoImpl{
			db:    repository.DB,
			cache: cache.NewCache[*model.PaymentConfig](paymentConfigCacheTTL, paymentConfigCacheCleanup),
		}
	})
	return paymentConfigDao
}

func paymentConfigCacheKey(payAddress string) string {
	return fmt.Sprintf(paymentConfigCacheKeyFmt, payAddress)
}

func (d *PaymentConfigDaoImpl) ListAll(ctx context.Context) ([]*model.PaymentConfig, error) {
	var configs []*model.PaymentConfig
	if err := d.db.WithContext(ctx).Order("id ASC").Find(&configs).Error; err != nil {
		log.WithContext(ctx).Errorf("failed to list payment configs: %v", err)
		return nil, err
	}
	return configs, nil
}

func (d *PaymentConfigDaoImpl) GetByPayAddress(ctx context.Context, payAddress string) (*model.PaymentConfig, error) {
	return d.cache.GetWithLoad(ctx, paymentConfigCacheKey(payAddress), func(ctx context.Context) (*model.PaymentConfig, error) {
		return d.getByPayAddressFromDB(ctx, payAddress)
	})
}

func (d *PaymentConfigDaoImpl) getByPayAddressFromDB(ctx context.Context, payAddress string) (*model.PaymentConfig, error) {
	var cfg model.PaymentConfig
	err := d.db.WithContext(ctx).
		Where("pay_address = ?", payAddress).
		First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		log.WithContext(ctx).Errorf("failed to get payment config: pay_address=%s err=%v", payAddress, err)
		return nil, err
	}
	return &cfg, nil
}
