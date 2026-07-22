package config

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type DatabaseType int

const (
	_ DatabaseType = iota
	Mysql
	Postgresql
	Sqlserver
)

type _Tls struct {
	Use  bool   `yaml:"use"`
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}
type _Mode struct {
	Id           int       `yaml:"id"`
	Port         string    `yaml:"port"`
	Tls          _Tls      `yaml:"tls"`
	UploadPath   string    `yaml:"path"`
	DocumentRoot string    `yaml:"documentRoot"`
	Mail         _Mail     `yaml:"mail"`
	Sms          _Sms      `yaml:"sms"`
	Cors         []string  `yaml:"cors"`
	Server       []string  `yaml:"server"`
	Database     _Database `yaml:"database"`
	Log          _Log      `yaml:"log"`
	JwtSecret    string    `yaml:"jwtSecret"`
	Apple        _Apple    `yaml:"apple"`
	Dash         _Dash     `yaml:"dash"`
}

// 대시보드 전용 시크릿 모음. .env.yml 값이 기본이고 환경변수가 덮어쓴다.
type _Dash struct {
	Token             string `yaml:"token"`             // 프론트 → API Bearer 토큰
	HealthIngestToken string `yaml:"healthIngestToken"` // Health Auto Export api-key 헤더
	GithubToken       string `yaml:"githubToken"`
	GithubUsername    string `yaml:"githubUsername"`
	GitlabToken       string `yaml:"gitlabToken"`
	GitlabUsername    string `yaml:"gitlabUsername"`
	SnippetEmail      string `yaml:"snippetEmail"`
	SnippetPassword   string `yaml:"snippetPassword"`
	SnippetApiUrl     string `yaml:"snippetApiUrl"`
}

type _Database struct {
	Host             string       `yaml:"host"`
	Port             string       `yaml:"port"`
	Name             string       `yaml:"name"`
	Owner            string       `yaml:"owner"`
	User             string       `yaml:"user"`
	Password         string       `yaml:"password"`
	Type             DatabaseType `yaml:"typeInner"`
	TypeString       string       `yaml:"type"`
	ConnectionString string       `yaml:"connectionString"`
}

type _Mail struct {
	Server   string `yaml:"server"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Sender   string `yaml:"sender"`
}

type _Sms struct {
	User   string `yaml:"user"`
	Key    string `yaml:"key"`
	Sender string `yaml:"sender"`
}

type _Log struct {
	Level    string `yaml:"level"`
	Console  bool   `yaml:"console"`
	Web      bool   `yaml:"web"`
	Database bool   `yaml:"database"`
	File     string `yaml:"file"`
	Limit    struct {
		Size  int `yaml:"size"`
		Count int `yaml:"count"`
		Days  int `yaml:"days"`
	} `yaml:"limit"`
}

type Config struct {
	Version    string `yaml:"version"`
	Develop    _Mode  `yaml:"develop"`
	Production _Mode  `yaml:"production"`
}

type _Apple struct {
	TeamID         string `yaml:"teamId"`         // Apple Developer Team ID (10 chars)
	KeyID          string `yaml:"keyId"`          // Key ID of the Sign in with Apple .p8 key
	ClientID       string `yaml:"clientId"`       // App Bundle ID (aud of identity token / sub of client secret)
	PrivateKeyPath string `yaml:"privateKeyPath"` // Path to the .p8 file
	PrivateKey     string `yaml:"privateKey"`     // Inline PEM contents (overrides path); also holds resolved key
}

var Mail _Mail
var Database _Database
var Sms _Sms
var Tls _Tls
var Log _Log
var Apple _Apple
var UploadPath string
var DocumentRoot string
var DashToken string
var HealthIngestToken string
var GithubToken string
var GithubUsername string
var GitlabToken string
var GitlabUsername string
var SnippetEmail string
var SnippetPassword string
var SnippetApiUrl string
var Version string
var Mode string
var Port string
var JwtSecret string
var Cors []string
var Server []string
var _value map[string]interface{}

var CrawlerId string

func Init() {
	config := &Config{}
	obj := make(map[string]interface{})

	buf, err := os.ReadFile(".env.yml")
	if err == nil {
		err = yaml.Unmarshal(buf, config)
		if err != nil {
			log.Println(err.Error())
		} else {
			err = yaml.Unmarshal(buf, obj)
			if err != nil {
				log.Println(err.Error())
			}
		}
	}

	Tls.Use = false

	Mode = os.Getenv("APP_MODE")

	if len(os.Args) == 3 {
		if os.Args[1] == "--mode" {
			Mode = os.Args[2]
		}
	}

	if Mode != "production" {
		Mode = "develop"
	}

	Log.Level = "debug"
	Log.Console = true
	Log.Web = true
	Log.Database = true
	Log.File = "webdata/log/system.log"

	if Mode == "production" {
		Mail = config.Production.Mail
		Sms = config.Production.Sms
		UploadPath = config.Production.UploadPath
		DocumentRoot = config.Production.DocumentRoot
		Port = config.Production.Port
		Database = config.Production.Database
		Cors = config.Production.Cors
		Log = config.Production.Log
		Server = config.Production.Server
		Tls = config.Production.Tls
		JwtSecret = config.Production.JwtSecret
		Apple = config.Production.Apple
		setDash(config.Production.Dash)

		if _, exist := obj["production"]; exist {
			_value = obj["production"].(map[string]interface{})
		}
	} else {
		Mail = config.Develop.Mail
		Sms = config.Develop.Sms
		UploadPath = config.Develop.UploadPath
		DocumentRoot = config.Develop.DocumentRoot
		Port = config.Develop.Port
		Database = config.Develop.Database
		Cors = config.Develop.Cors
		Log = config.Develop.Log
		Server = config.Develop.Server
		Tls = config.Develop.Tls
		JwtSecret = config.Develop.JwtSecret
		Apple = config.Develop.Apple
		setDash(config.Develop.Dash)

		if _, exist := obj["develop"]; exist {
			_value = obj["develop"].(map[string]interface{})
		}
	}

	if DocumentRoot == "" {
		DocumentRoot = "dist"
	}

	envPort := os.Getenv("PORT")
	if envPort != "" {
		Port = envPort
	}

	envLogLevel := os.Getenv("LOG_LEVEL")
	envLogConsole := os.Getenv("LOG_CONSOLE")
	envLogWeb := os.Getenv("LOG_WEB")
	envLogDatabase := os.Getenv("LOG_DATABASE")
	envLogFile := os.Getenv("LOG_FILE")
	envLogDays := os.Getenv("LOG_DAYS")
	if envLogLevel != "" {
		Log.Level = envLogLevel
	}
	if envLogConsole != "" {
		if envLogConsole == "Y" {
			Log.Console = true
		}
	}
	if envLogWeb != "" {
		if envLogWeb == "Y" {
			Log.Web = true
		}
	}
	if envLogDatabase != "" {
		if envLogDatabase == "Y" {
			Log.Database = true
		}
	}
	if envLogFile != "" {
		Log.File = envLogFile
	}
	if envLogDays != "" {
		days, _ := strconv.Atoi(envLogDays)
		if days == 0 {
			Log.File = ""
		}
		Log.Limit.Days = days
	}

	Log.Level = "debug"
	Log.Console = true
	Log.Web = true
	// Log.Database(SQL 파라미터 디버그 로깅)는 강제하지 않는다. 활성화 시 bcrypt 해시 등
	// 민감한 파라미터가 로그 파일에 기록되므로, .env.yml(log.database) / LOG_DATABASE=Y 로만 opt-in 한다.
	Log.File = "webdata/log/system.log"
	envDBType := os.Getenv("DB_TYPE")
	envDBHost := os.Getenv("DB_HOST")
	envDBPort := os.Getenv("DB_PORT")
	envDBName := os.Getenv("DB_NAME")
	envDBUser := os.Getenv("DB_USER")
	envDBPass := os.Getenv("DB_PASS")
	if envDBType != "" {
		Database.TypeString = envDBType
	}
	if envDBHost != "" {
		Database.Host = envDBHost
	}
	if envDBPort != "" {
		Database.Port = envDBPort
	}
	if envDBName != "" {
		Database.Name = envDBName
	}
	if envDBUser != "" {
		Database.User = envDBUser
	}
	if envDBPass != "" {
		Database.Password = envDBPass
	}

	envMailServer := os.Getenv("MAIL_SERVER")
	envMailPort := os.Getenv("MAIL_PORT")
	envMailUser := os.Getenv("MAIL_USER")
	envMailPass := os.Getenv("MAIL_PASS")
	envMailSender := os.Getenv("MAIL_SENDER")
	if envMailServer != "" {
		Mail.Server = envMailServer
	}
	if envMailPort != "" {
		Mail.Port = envMailPort
	}
	if envMailUser != "" {
		Mail.User = envMailUser
	}
	if envMailPass != "" {
		Mail.Password = envMailPass
	}
	if envMailSender != "" {
		Mail.Sender = envMailSender
	}

	envTlsCert := strings.ToUpper(os.Getenv("TLS_CERT"))
	if envTlsCert != "" {
		Tls.Cert = envTlsCert
	}
	envTlsKey := strings.ToUpper(os.Getenv("TLS_KEY"))
	if envTlsKey != "" {
		Tls.Key = envTlsKey
	}
	envTlsUse := strings.ToUpper(os.Getenv("TLS_USE"))
	if envTlsUse == "TRUE" || envTlsUse == "T" || envTlsUse == "YES" || envTlsUse == "Y" {
		Tls.Use = true
		if Tls.Cert == "" {
			Tls.Cert = path.Join(UploadPath + "certs/ssl.crt")
		}
		if Tls.Key == "" {
			Tls.Key = path.Join(UploadPath + "certs/ssl.key")
		}
	}

	if Port == "" {
		Port = "80"
	}

	if UploadPath == "" {
		UploadPath = "webdata"
	}

	if Database.TypeString == "postgres" || Database.TypeString == "postgresql" {
		if Database.Port == "" {
			Database.Port = "5432"
		}

		Database.Type = Postgresql
	} else if Database.TypeString == "sqlserver" || Database.TypeString == "mssql" {
		if Database.Port == "" {
			Database.Port = "1433"
		}

		Database.Type = Sqlserver
	} else {
		if Database.Port == "" {
			Database.Port = "3306"
		}

		Database.Type = Mysql
	}

	if Database.ConnectionString == "" {
		if Database.Type == Postgresql {
			Database.ConnectionString = fmt.Sprintf("host=%v port=%v user=%v password=%v dbname=%v sslmode=disable", Database.Host, Database.Port, Database.User, Database.Password, Database.Name)
		} else if Database.Type == Sqlserver {
			Database.ConnectionString = fmt.Sprintf("server=%v;port=%v;user id=%v,password=%v;database=%v", Database.Host, Database.Port, Database.User, Database.Password, Database.Name)
		} else {
			Database.ConnectionString = fmt.Sprintf("%v:%v@tcp(%v:%v)/%v", Database.User, Database.Password, Database.Host, Database.Port, Database.Name)
		}
	}

	// Sign in with Apple — used for server-to-server token revocation on
	// account deletion. Base values come from .env.yml (per mode); environment
	// variables override them (handy for Docker/production secrets).
	if v := os.Getenv("APPLE_TEAM_ID"); v != "" {
		Apple.TeamID = v
	}
	if v := os.Getenv("APPLE_KEY_ID"); v != "" {
		Apple.KeyID = v
	}
	if v := os.Getenv("APPLE_CLIENT_ID"); v != "" {
		Apple.ClientID = v
	}
	if Apple.ClientID == "" {
		Apple.ClientID = "com.gowoobro.fotstat"
	}
	if v := os.Getenv("APPLE_PRIVATE_KEY_PATH"); v != "" {
		Apple.PrivateKeyPath = v
	}
	if v := os.Getenv("APPLE_PRIVATE_KEY"); v != "" {
		// Inline PEM. Allow "\n" escapes so it can be passed on one line.
		Apple.PrivateKey = strings.ReplaceAll(v, "\\n", "\n")
	}
	// Resolve the private key: inline value wins, otherwise read from the path.
	if Apple.PrivateKey == "" && Apple.PrivateKeyPath != "" {
		if buf, err := os.ReadFile(Apple.PrivateKeyPath); err == nil {
			Apple.PrivateKey = string(buf)
		} else {
			log.Printf("apple private key read failed: %v", err)
		}
	}

	// 대시보드 시크릿 환경변수 오버라이드 (Docker/운영용)
	if v := os.Getenv("DASH_TOKEN"); v != "" {
		DashToken = v
	}
	if v := os.Getenv("HEALTH_INGEST_TOKEN"); v != "" {
		HealthIngestToken = v
	}
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		GithubToken = v
	}
	if v := os.Getenv("GITHUB_USERNAME"); v != "" {
		GithubUsername = v
	}
	if v := os.Getenv("GITLAB_TOKEN"); v != "" {
		GitlabToken = v
	}
	if v := os.Getenv("GITLAB_USERNAME"); v != "" {
		GitlabUsername = v
	}
	if v := os.Getenv("SNIPPET_EMAIL"); v != "" {
		SnippetEmail = v
	}
	if v := os.Getenv("SNIPPET_PASSWORD"); v != "" {
		SnippetPassword = v
	}
	if v := os.Getenv("SNIPPET_API_URL"); v != "" {
		SnippetApiUrl = v
	}
	if GithubUsername == "" {
		GithubUsername = "kobums23"
	}
	if SnippetApiUrl == "" {
		SnippetApiUrl = "https://snippetapi.gowoobro.com/api"
	}

	Version = config.Version
	CrawlerId = "chin1525"
}

func setDash(d _Dash) {
	DashToken = d.Token
	HealthIngestToken = d.HealthIngestToken
	GithubToken = d.GithubToken
	GithubUsername = d.GithubUsername
	GitlabToken = d.GitlabToken
	GitlabUsername = d.GitlabUsername
	SnippetEmail = d.SnippetEmail
	SnippetPassword = d.SnippetPassword
	SnippetApiUrl = d.SnippetApiUrl
}

// AppleConfigured reports whether enough Apple credentials are present to
// perform server-to-server token exchange and revocation.
func AppleConfigured() bool {
	return Apple.TeamID != "" && Apple.KeyID != "" && Apple.ClientID != "" && Apple.PrivateKey != ""
}

func Get(name string) interface{} {
	return _value[name]
}

func GetString(name string) string {
	return _value[name].(string)
}

func GetInt(name string) int {
	return _value[name].(int)
}
