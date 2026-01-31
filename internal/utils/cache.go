package utils

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache 表示一个简单的文件缓存
// 用于缓存文件内容和处理结果，避免重复处理
// 适用于知识库录入场景

type Cache struct {
	cacheDir string
	mutex    sync.RWMutex
}

// NewCache 创建一个新的缓存实例
// cacheDir 是缓存文件存储的目录
func NewCache(cacheDir string) (*Cache, error) {
	// 确保缓存目录存在
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %v", err)
	}

	return &Cache{
		cacheDir: cacheDir,
	}, nil
}

// getCacheKey 生成缓存键
func (c *Cache) getCacheKey(key string) string {
	hash := md5.Sum([]byte(key))
	return hex.EncodeToString(hash[:])
}

// getCachePath 生成缓存文件路径
func (c *Cache) getCachePath(key string) string {
	cacheKey := c.getCacheKey(key)
	return filepath.Join(c.cacheDir, cacheKey+"json")
}

// Set 将数据存储到缓存中
func (c *Cache) Set(key string, value interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 生成缓存文件路径
	cachePath := c.getCachePath(key)

	// 序列化数据
	data, err := json.Marshal(map[string]interface{}{
		"value":     value,
		"timestamp": time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %v", err)
	}

	// 写入缓存文件
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %v", err)
	}

	return nil
}

// Get 从缓存中获取数据
func (c *Cache) Get(key string, value interface{}) (bool, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 生成缓存文件路径
	cachePath := c.getCachePath(key)

	// 检查缓存文件是否存在
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return false, nil // 缓存不存在
	}

	// 读取缓存文件
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return false, fmt.Errorf("failed to read cache file: %v", err)
	}

	// 解析缓存数据
	var cachedData map[string]interface{}
	if err := json.Unmarshal(data, &cachedData); err != nil {
		return false, fmt.Errorf("failed to unmarshal cache data: %v", err)
	}

	// 反序列化值
	valueData, err := json.Marshal(cachedData["value"])
	if err != nil {
		return false, fmt.Errorf("failed to marshal cached value: %v", err)
	}

	if err := json.Unmarshal(valueData, value); err != nil {
		return false, fmt.Errorf("failed to unmarshal cached value: %v", err)
	}

	return true, nil
}

// Delete 从缓存中删除数据
func (c *Cache) Delete(key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 生成缓存文件路径
	cachePath := c.getCachePath(key)

	// 删除缓存文件
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %v", err)
	}

	return nil
}

// Clear 清空所有缓存
func (c *Cache) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 遍历缓存目录
	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %v", err)
	}

	// 删除所有缓存文件
	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(c.cacheDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to delete cache file %s: %v", file.Name(), err)
			}
		}
	}

	return nil
}

// GetFileCacheKey 生成文件缓存键
func GetFileCacheKey(filePath string) string {
	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Sprintf("file:%s", filePath)
	}

	// 使用文件路径、修改时间和大小生成缓存键
	return fmt.Sprintf("file:%s:%d:%d", filePath, fileInfo.ModTime().Unix(), fileInfo.Size())
}