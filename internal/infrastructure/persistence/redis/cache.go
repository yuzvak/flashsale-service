package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/bloom"
	"github.com/yuzvak/flashsale-service/internal/infrastructure/monitoring"
	"github.com/yuzvak/flashsale-service/internal/pkg/logger"
)

type Cache struct {
	client      *redis.Client
	bloomFilter *bloom.RedisBloomFilter
	logger      *logger.Logger

	purchaseScript  *redis.Script
	userLimitScript *redis.Script
	saleLimitScript *redis.Script
}

func NewCache(conn *Connection, log *logger.Logger) *Cache {
	client := monitoring.InstrumentRedisClient(conn.GetClient())

	m, k := bloom.GetOptimalParameters(100000, 0.01)
	bloomFilter := bloom.NewRedisBloomFilter(client, "bloom:sold_items", m, k)

	return &Cache{
		client:          client,
		bloomFilter:     bloomFilter,
		logger:          log,
		purchaseScript:  redis.NewScript(purchaseLuaScript),
		userLimitScript: redis.NewScript(userLimitLuaScript),
		saleLimitScript: redis.NewScript(saleLimitLuaScript),
	}
}


func (c *Cache) AddItemToBloomFilter(ctx context.Context, itemID string) error {
	return c.bloomFilter.Add(ctx, itemID)
}

func (c *Cache) ItemExistsInBloomFilter(ctx context.Context, itemID string) (bool, error) {
	return c.bloomFilter.Contains(ctx, itemID)
}


func (c *Cache) GetUserItemCount(ctx context.Context, saleID, userID string) (int, error) {
	key := fmt.Sprintf("user:%s:sale:%s:count", userID, saleID)
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	count, err := strconv.Atoi(result)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (c *Cache) IncrementUserItemCount(ctx context.Context, saleID, userID string) error {
	key := fmt.Sprintf("user:%s:sale:%s:count", userID, saleID)
	_, err := c.client.Incr(ctx, key).Result()
	return err
}

func (c *Cache) GetUserCheckoutCount(ctx context.Context, saleID, userID string) (int, error) {
	key := fmt.Sprintf("user:%s:sale:%s:checkout_count", userID, saleID)
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	count, err := strconv.Atoi(result)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (c *Cache) IncrementUserCheckoutCount(ctx context.Context, saleID, userID string) error {
	key := fmt.Sprintf("user:%s:sale:%s:checkout_count", userID, saleID)
	_, err := c.client.Incr(ctx, key).Result()
	return err
}

func (c *Cache) SetUserCheckoutCount(ctx context.Context, saleID, userID string, count int, expiration time.Duration) error {
	key := fmt.Sprintf("user:%s:sale:%s:checkout_count", userID, saleID)
	return c.client.Set(ctx, key, count, expiration).Err()
}

func (c *Cache) GetAvailableCheckoutSlots(ctx context.Context, saleID, userID string, maxItems int) (int, error) {
	purchasedCount, err := c.GetUserItemCount(ctx, saleID, userID)
	if err != nil {
		return 0, err
	}

	checkoutCount, err := c.GetUserCheckoutCount(ctx, saleID, userID)
	if err != nil {
		return 0, err
	}

	return maxItems - purchasedCount - checkoutCount, nil
}

func (c *Cache) SetUserItemCount(ctx context.Context, saleID, userID string, count int, expiration time.Duration) error {
	key := fmt.Sprintf("user:%s:sale:%s:count", userID, saleID)
	return c.client.Set(ctx, key, count, expiration).Err()
}


func (c *Cache) GetUserCheckoutCode(ctx context.Context, saleID, userID string) (string, error) {
	key := fmt.Sprintf("user:%s:sale:%s:checkout", userID, saleID)
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}

	return result, nil
}

func (c *Cache) SetUserCheckoutCode(ctx context.Context, saleID, userID, code string, expiration time.Duration) error {
	key := fmt.Sprintf("user:%s:sale:%s:checkout", userID, saleID)
	return c.client.Set(ctx, key, code, expiration).Err()
}

func (c *Cache) RemoveUserCheckoutCode(ctx context.Context, saleID, userID string) error {
	checkoutKey := fmt.Sprintf("user:%s:sale:%s:checkout", userID, saleID)
	checkoutCountKey := fmt.Sprintf("user:%s:sale:%s:checkout_count", userID, saleID)

	pipe := c.client.Pipeline()
	pipe.Del(ctx, checkoutKey)
	pipe.Del(ctx, checkoutCountKey)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Cache) SetCheckoutCode(ctx context.Context, code string, expiration time.Duration) error {
	key := fmt.Sprintf("checkout:%s", code)
	return c.client.Set(ctx, key, "1", expiration).Err()
}

func (c *Cache) CheckoutCodeExists(ctx context.Context, code string) (bool, error) {
	key := fmt.Sprintf("checkout:%s", code)
	result, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return result > 0, nil
}

func (c *Cache) RemoveCheckoutCode(ctx context.Context, code string) error {
	key := fmt.Sprintf("checkout:%s", code)
	return c.client.Del(ctx, key).Err()
}

func (c *Cache) HasUserCheckedOutItem(ctx context.Context, saleID, userID, itemID string) (bool, error) {
	key := fmt.Sprintf("user:%s:sale:%s:checked_items", userID, saleID)
	result, err := c.client.SIsMember(ctx, key, itemID).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (c *Cache) AddUserCheckedOutItem(ctx context.Context, saleID, userID, itemID string, expiration time.Duration) error {
	key := fmt.Sprintf("user:%s:sale:%s:checked_items", userID, saleID)

	pipe := c.client.Pipeline()
	pipe.SAdd(ctx, key, itemID)
	pipe.Expire(ctx, key, expiration)

	_, err := pipe.Exec(ctx)
	return err
}


func (c *Cache) IncrementSaleItemsSold(ctx context.Context, saleID string, count int) error {
	key := fmt.Sprintf("sale:%s:items_sold", saleID)
	_, err := c.client.IncrBy(ctx, key, int64(count)).Result()
	return err
}

func (c *Cache) GetSaleItemsSold(ctx context.Context, saleID string) (int, error) {
	key := fmt.Sprintf("sale:%s:items_sold", saleID)
	result, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	count, err := strconv.Atoi(result)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (c *Cache) AtomicPurchaseCheck(ctx context.Context, saleID, userID string, itemCount int, maxSaleItems, maxUserItems int) (bool, error) {
	keys := []string{
		fmt.Sprintf("sale:%s:items_sold", saleID),
		fmt.Sprintf("user:%s:sale:%s:count", userID, saleID),
	}
	args := []interface{}{itemCount, maxSaleItems, maxUserItems}
	c.logger.Info("AtomicPurchaseCheck input", "keys", keys, "args", args)

	result, err := c.purchaseScript.Run(ctx, c.client, keys, args...).Result()
	if err != nil {
		c.logger.Error("AtomicPurchaseCheck script error", "error", err)
		return false, err
	}

	resultInt := result.(int64)
	c.logger.Info("AtomicPurchaseCheck result", "lua_result", resultInt, "can_purchase", resultInt == 1)

	return resultInt == 1, nil
}

func (c *Cache) AtomicUserLimitCheck(ctx context.Context, saleID, userID string, itemCount, maxItems int) (bool, error) {
	keys := []string{fmt.Sprintf("user:%s:sale:%s:count", userID, saleID)}
	args := []interface{}{itemCount, maxItems}

	result, err := c.userLimitScript.Run(ctx, c.client, keys, args...).Result()
	if err != nil {
		return false, err
	}

	return result.(int64) == 1, nil
}

func (c *Cache) AtomicSaleLimitCheck(ctx context.Context, saleID string, itemCount, maxItems int) (bool, error) {
	keys := []string{fmt.Sprintf("sale:%s:items_sold", saleID)}
	args := []interface{}{itemCount, maxItems}

	result, err := c.saleLimitScript.Run(ctx, c.client, keys, args...).Result()
	if err != nil {
		return false, err
	}

	return result.(int64) == 1, nil
}

func (c *Cache) DistributedLock(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("lock:%s", key)
	result, err := c.client.SetNX(ctx, lockKey, "1", expiration).Result()
	if err == nil {
		if result {
			monitoring.RedisLockSuccessTotal.WithLabelValues(key).Inc()
		} else {
			monitoring.RedisLockFailureTotal.WithLabelValues(key, "already_locked").Inc()
		}
	} else {
		monitoring.RedisLockFailureTotal.WithLabelValues(key, "redis_error").Inc()
	}
	return result, err
}

func (c *Cache) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("lock:%s", key)
	err := c.client.Del(ctx, lockKey).Err()
	return err
}

const purchaseLuaScript = `
	local sale_key = KEYS[1]
	local user_key = KEYS[2]
	local item_count = tonumber(ARGV[1])
	local max_sale_items = tonumber(ARGV[2])
	local max_user_items = tonumber(ARGV[3])

	-- Get current counts
	local current_sale_count = tonumber(redis.call('GET', sale_key) or 0)
	local current_user_count = tonumber(redis.call('GET', user_key) or 0)

	-- Log debug info
	redis.log(redis.LOG_WARNING, 'LUA DEBUG: sale_key=' .. sale_key .. ', user_key=' .. user_key)
	redis.log(redis.LOG_WARNING, 'LUA DEBUG: item_count=' .. item_count .. ', max_sale_items=' .. max_sale_items .. ', max_user_items=' .. max_user_items)
	redis.log(redis.LOG_WARNING, 'LUA DEBUG: current_sale_count=' .. current_sale_count .. ', current_user_count=' .. current_user_count)

	-- Check limits
	if current_sale_count + item_count > max_sale_items then
		redis.log(redis.LOG_WARNING, 'LUA DEBUG: Sale limit exceeded: ' .. (current_sale_count + item_count) .. ' > ' .. max_sale_items)
		return 0  -- Sale limit exceeded
	end

	-- For user limit, check if user has enough remaining capacity
	local remaining_user_capacity = max_user_items - current_user_count
	redis.log(redis.LOG_WARNING, 'LUA DEBUG: remaining_user_capacity=' .. remaining_user_capacity)
	if item_count > remaining_user_capacity then
		redis.log(redis.LOG_WARNING, 'LUA DEBUG: User limit exceeded: ' .. item_count .. ' > ' .. remaining_user_capacity)
		return 0  -- User limit exceeded
	end

	-- Increment both sale and user counters
	redis.call('INCRBY', sale_key, item_count)
	redis.call('INCRBY', user_key, item_count)
	redis.log(redis.LOG_WARNING, 'LUA DEBUG: Purchase successful, incremented sale counter by ' .. item_count .. ' and user counter by ' .. item_count)

	return 1  -- Success
	`

const userLimitLuaScript = `
	local user_key = KEYS[1]
	local item_count = tonumber(ARGV[1])
	local max_items = tonumber(ARGV[2])

	local current_count = tonumber(redis.call('GET', user_key) or 0)

	if current_count + item_count > max_items then
		return 0  -- Limit exceeded
	end

	redis.call('INCRBY', user_key, item_count)
	redis.call('EXPIRE', user_key, 86400)  -- 24 hours

	return 1  -- Success
	`

const saleLimitLuaScript = `
	local sale_key = KEYS[1]
	local item_count = tonumber(ARGV[1])
	local max_items = tonumber(ARGV[2])

	local current_count = tonumber(redis.call('GET', sale_key) or 0)

	if current_count + item_count > max_items then
		return 0  -- Limit exceeded
	end

	redis.call('INCRBY', sale_key, item_count)

	return 1  -- Success
`

func (c *Cache) DecrementCounters(ctx context.Context, saleID, userID string, itemCount int) error {
	keys := []string{
		fmt.Sprintf("sale:%s:items_sold", saleID),
		fmt.Sprintf("user:%s:sale:%s:count", userID, saleID),
	}
	args := []interface{}{itemCount}

	decrementScript := redis.NewScript(`
		local sale_key = KEYS[1]
		local user_key = KEYS[2]
		local item_count = tonumber(ARGV[1])

		-- Decrement both counters, but don't go below 0
		local current_sale_count = tonumber(redis.call('GET', sale_key) or 0)
		local current_user_count = tonumber(redis.call('GET', user_key) or 0)

		local new_sale_count = math.max(0, current_sale_count - item_count)
		local new_user_count = math.max(0, current_user_count - item_count)

		redis.call('SET', sale_key, new_sale_count)
		redis.call('SET', user_key, new_user_count)

		return 1
	`)

	_, err := decrementScript.Run(ctx, c.client, keys, args...).Result()
	return err
}

func (c *Cache) GetSaleItemCount(ctx context.Context, saleID string) (int, error) {
	key := fmt.Sprintf("sale:%s:items_sold", saleID)
	result, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	count, err := strconv.Atoi(result)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (c *Cache) IncrementCounters(ctx context.Context, saleID, userID string, itemCount int) error {
	keys := []string{
		fmt.Sprintf("sale:%s:items_sold", saleID),
		fmt.Sprintf("user:%s:sale:%s:count", userID, saleID),
	}
	args := []interface{}{itemCount}

	incrementScript := redis.NewScript(`
		local sale_key = KEYS[1]
		local user_key = KEYS[2]
		local item_count = tonumber(ARGV[1])

		-- Increment both counters
		redis.call('INCRBY', sale_key, item_count)
		redis.call('INCRBY', user_key, item_count)
		redis.call('EXPIRE', user_key, 86400)  -- 24 hours

		return 1
	`)

	_, err := incrementScript.Run(ctx, c.client, keys, args...).Result()
	return err
}
