package config

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type MainConfig struct {
	AppName string `toml:"appName"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}

type MysqlConfig struct {
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	User         string `toml:"user"`
	Password     string `toml:"password"`
	DatabaseName string `toml:"databaseName"`
}

type RedisConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Password string `toml:"password"`
	Db       int    `toml:"db"`
}

type AuthCodeConfig struct {
	AccessKeyID     string `toml:"accessKeyID"`
	AccessKeySecret string `toml:"accessKeySecret"`
	SignName        string `toml:"signName"`
	TemplateCode    string `toml:"templateCode"`
}

type LogConfig struct {
	LogPath string `toml:"logPath"`
}

type KafkaConfig struct {
	MessageMode string        `toml:"messageMode"`
	HostPort    string        `toml:"hostPort"`
	LoginTopic  string        `toml:"loginTopic"`
	LogoutTopic string        `toml:"logoutTopic"`
	ChatTopic   string        `toml:"chatTopic"`
	Partition   int           `toml:"partition"`
	Timeout     time.Duration `toml:"timeout"`
}

type StaticSrcConfig struct {
	StaticAvatarPath string `toml:"staticAvatarPath"`
	StaticFilePath   string `toml:"staticFilePath"`
}

type Config struct {
	MainConfig      `toml:"mainConfig"`
	MysqlConfig     `toml:"mysqlConfig"`
	RedisConfig     `toml:"redisConfig"`
	AuthCodeConfig  `toml:"authCodeConfig"`
	LogConfig       `toml:"logConfig"`
	KafkaConfig     `toml:"kafkaConfig"`
	StaticSrcConfig `toml:"staticSrcConfig"`
}

var config *Config

// TODO: 修改项目路径和部署方式
func LoadConfig() error {
	if config == nil {
		config = new(Config)
	}

	// // 本地部署
	// // if _, err := toml.DecodeFile("F:\\go\\kama-chat-server\\configs\\config_local.toml", config); err != nil {
	// // 	log.Fatal(err.Error())
	// // 	return err
	// // }
	// // Ubuntu22.04云服务器部署
	// if _, err := toml.DecodeFile("/root/project/KamaChat/configs/config_local.toml", config); err != nil {
	// 	log.Fatal(err.Error())
	// 	return err
	// }
	// return nil

	var configPath string
	// 优先读取环境变量 Docker 等
	if envPath := os.Getenv("GO_CHAT_CONFIG_PATH"); envPath != "" {
		configPath = envPath
	} else { // 读取本地配置
		exeDir, err := os.Executable()
		if err != nil {
			log.Printf("获取程序目录失败，使用当前工作目录：%s", err)
			exeDir, _ = os.Getwd()
		}
		// 拼接配置文件路径
		configPath = filepath.Join(filepath.Dir(exeDir), "configs", "config_local.toml")
	}
	log.Printf("正在加载配置文件: %s", configPath)
	// 解码配置文件
	_, err := toml.DecodeFile(configPath, config)
	if err != nil {
		log.Printf("加载配置文件失败: %s", err.Error())
		return err
	}

	log.Println("配置文件加载成功")
	return nil
}

func GetConfig() *Config {
	// if config == nil {
	// 	config = new(Config)
	// 	_ = LoadConfig()
	// }
	// return config
	if config == nil {
		config = new(Config)
		if err := LoadConfig(); err != nil {
			log.Fatalf("初始化配置失败，程序退出: %s", err.Error())
		}
	}
	return config
}
