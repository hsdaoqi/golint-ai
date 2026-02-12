package config

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	AI struct {
		APIKey     string `mapstructure:"api_key"`
		APIURL     string `mapstructure:"api_url"`
		Model      string `mapstructure:"model"`
		MaxRetries int    `mapstructure:"max_retries"`
	} `mapstructure:"ai"`
}

var (
	GlobalConfig *Config
	once         sync.Once
)

func Load() *Config {
	once.Do(func() {
		// 1. 设置配置文件信息
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")

		// 2. 【核心】环境变量配置
		viper.SetEnvPrefix("GOLINT") // 设置环境变量前缀为 GOLINT_
		viper.AutomaticEnv()         // 自动读取环境变量

		// 3. 【重点】处理嵌套结构
		// 将 YAML 中的 ai.api_key 映射为环境变量 GOLINT_AI_API_KEY
		// Viper 默认不处理点号到下划线的转换，需要手动设置
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

		// 4. 读取配置文件（如果不存在也行，因为可能全靠环境变量）
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				panic(fmt.Errorf("读取配置文件出错: %w", err))
			}
			// 如果没找到文件，没关系，我们可能在生产环境用环境变量
		}

		GlobalConfig = &Config{}
		if err := viper.Unmarshal(GlobalConfig); err != nil {
			panic(err)
		}
	})
	return GlobalConfig
}
