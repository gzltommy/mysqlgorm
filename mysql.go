package mysqlgorm

import (
	"context"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/ssh"
	sql "gorm.io/driver/mysql" // gorm 库也是使用的 github.com/go-sql-driver/mysql 作为驱动
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"net"
	"os"
	"time"
)

type SshKeyType = string

const (
	SSHKeyTypeKey      SshKeyType = "KEY"
	SSHKeyTypePassword SshKeyType = "PASSWORD"
)

type SSHConfig struct {
	Host     string
	User     string
	Port     string
	KeyType  SshKeyType
	Password string
	KeyFile  string
	TimeOut  time.Duration
}

func (sc *SSHConfig) dialWithPassword() (*ssh.Client, error) {
	if sc.TimeOut == 0 {
		sc.TimeOut = time.Second * 15
	}

	config := &ssh.ClientConfig{
		User: sc.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(sc.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sc.TimeOut,
	}
	return ssh.Dial("tcp", net.JoinHostPort(sc.Host, sc.Port), config)
}

func (sc *SSHConfig) dialWithKeyFile() (*ssh.Client, error) {
	if sc.TimeOut == 0 {
		sc.TimeOut = time.Second * 15
	}
	config := &ssh.ClientConfig{
		User:            sc.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sc.TimeOut,
	}
	if k, err := os.ReadFile(sc.KeyFile); err != nil {
		return nil, err
	} else {
		signer, err := ssh.ParsePrivateKey(k)
		if err != nil {
			return nil, err
		}
		config.Auth = []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		}
	}
	return ssh.Dial("tcp", net.JoinHostPort(sc.Host, sc.Port), config)
}

type SQLConfig struct {
	Host           string `json:"host"`
	User           string `json:"user"`
	Port           string `json:"port"`
	Password       string `json:"password"`
	Database       string `json:"database"`
	MaxOpenConn    int    `json:"max_open_conn"`   // 默认 100
	MaxIdleConn    int    `json:"max_idle_conn"`   // 默认 20
	PluralityTable bool   `json:"plurality_table"` // 默认 false 单数；true：复数
}

type SQLClient struct {
	db        *gorm.DB
	sshClient *ssh.Client
}

func (m *SQLClient) DB() *gorm.DB {
	return m.db
}

func (m *SQLClient) Close() {
	if m.db != nil {
		idb, err := m.db.DB()
		if err == nil {
			idb.Close()
		}
	}
	if m.sshClient != nil {
		m.sshClient.Close()
	}
}

func NewMySQLClient(sqlC *SQLConfig, sshC *SSHConfig) (*SQLClient, error) {
	var (
		dsn       string
		sshClient *ssh.Client
		err       error
	)
	if sshC == nil {
		// 直接连接 SQLConfig 服务器
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			sqlC.User, sqlC.Password, sqlC.Host, sqlC.Port, sqlC.Database)
	} else {
		// 注意这里与上面的区别： 【tcp】 更换为【 mysql+ssh 】
		dsn = fmt.Sprintf("%s:%s@mysql+ssh(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			sqlC.User, sqlC.Password, sqlC.Host, sqlC.Port, sqlC.Database)

		switch sshC.KeyType {
		case SSHKeyTypeKey:
			sshClient, err = sshC.dialWithKeyFile()
		case SSHKeyTypePassword:
			sshClient, err = sshC.dialWithPassword()
		default:
			return nil, fmt.Errorf("unknown ssh type")
		}
		if err != nil {
			return nil, fmt.Errorf("ssh connect error: %w", err)
		}
		// 注册 ssh 代理
		mysql.RegisterDialContext("mysql+ssh", func(ctx context.Context, addr string) (net.Conn, error) {
			return sshClient.Dial("tcp", addr)
		})
	}
	db, err := gorm.Open(
		sql.Open(dsn),
		&gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
			NamingStrategy: schema.NamingStrategy{
				//TablePrefix:   "",
				SingularTable: !sqlC.PluralityTable, // 关闭默认表名是复数的方式
				//NameReplacer:  nil,
				//NoLowerCase:   false,
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("mysql connect error: %w", err)
	}

	//设置数据库连接池参数
	maxOpenConn := 100
	maxIdleConn := 20
	if sqlC.MaxOpenConn > 0 {
		maxOpenConn = sqlC.MaxOpenConn
	}
	if sqlC.MaxIdleConn > 0 {
		maxIdleConn = sqlC.MaxIdleConn
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("db.DB() fail.%w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConn) //设置数据库连接池最大连接数
	sqlDB.SetMaxIdleConns(maxIdleConn) //连接池最大允许的空闲连接数，如果没有sql任务需要执行的连接数大于20，超过的连接会被连接池关闭。
	sqlDB.SetConnMaxIdleTime(120 * time.Second)
	sqlDB.SetConnMaxLifetime(7200 * time.Second)
	return &SQLClient{
		db:        db,
		sshClient: sshClient,
	}, nil
}
